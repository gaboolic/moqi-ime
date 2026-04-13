package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"

	moqipb "github.com/gaboolic/moqi-ime/proto"
	gproto "google.golang.org/protobuf/proto"
)

func readFrame(reader *bufio.Reader) ([]byte, error) {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(reader, sizeBuf[:]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(sizeBuf[:])
	if size == 0 {
		return nil, fmt.Errorf("invalid empty frame")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeFrame(writer io.Writer, payload []byte) error {
	var sizeBuf [4]byte
	binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(payload)))
	if _, err := writer.Write(sizeBuf[:]); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

func decodeClientRequest(payload []byte) (*moqipb.ClientRequest, error) {
	msg := &moqipb.ClientRequest{}
	if err := gproto.Unmarshal(payload, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func encodeServerResponse(msg *moqipb.ServerResponse) ([]byte, error) {
	return gproto.Marshal(msg)
}
