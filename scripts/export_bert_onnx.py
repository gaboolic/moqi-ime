#!/usr/bin/env python3
"""Export a HuggingFace sequence-classification BERT model to ONNX for Moqi."""

from __future__ import annotations

import argparse
from pathlib import Path

import torch
from transformers import AutoModelForSequenceClassification, AutoTokenizer


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Export a HuggingFace sequence-classification model to ONNX for Moqi's BERT reranker."
    )
    parser.add_argument(
        "--model",
        required=True,
        help="Model name or local directory accepted by AutoModelForSequenceClassification.",
    )
    parser.add_argument(
        "--output-dir",
        required=True,
        help="Directory that will receive model.onnx and vocab/tokenizer files.",
    )
    parser.add_argument(
        "--opset",
        type=int,
        default=17,
        help="ONNX opset version. Default: 17.",
    )
    parser.add_argument(
        "--max-length",
        type=int,
        default=96,
        help="Dummy export sequence length. Default: 96.",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    output_dir = Path(args.output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)

    tokenizer = AutoTokenizer.from_pretrained(args.model)
    model = AutoModelForSequenceClassification.from_pretrained(args.model)
    model.eval()

    encoded = tokenizer(
        "今天的天气真不错",
        "今天的天气真的很好",
        max_length=args.max_length,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    )

    input_names = ["input_ids", "attention_mask"]
    input_tensors = (encoded["input_ids"], encoded["attention_mask"])
    dynamic_axes = {
        "input_ids": {0: "batch_size", 1: "sequence_length"},
        "attention_mask": {0: "batch_size", 1: "sequence_length"},
        "logits": {0: "batch_size"},
    }
    if "token_type_ids" in encoded:
        input_names.append("token_type_ids")
        input_tensors = input_tensors + (encoded["token_type_ids"],)
        dynamic_axes["token_type_ids"] = {0: "batch_size", 1: "sequence_length"}

    onnx_path = output_dir / "model.onnx"
    torch.onnx.export(
        model,
        input_tensors,
        onnx_path.as_posix(),
        input_names=input_names,
        output_names=["logits"],
        dynamic_axes=dynamic_axes,
        opset_version=args.opset,
    )

    tokenizer.save_pretrained(output_dir)
    print(f"Exported ONNX model to {onnx_path}")
    print("Update input_methods/rime/bert_config.json or APPDATA config paths if needed.")


if __name__ == "__main__":
    main()
