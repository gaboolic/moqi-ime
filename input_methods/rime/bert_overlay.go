package rime

import (
	"strings"

	"github.com/gaboolic/moqi-ime/imecore"
)

func (ime *IME) currentBertOverlay() (rimeState, []int, bool) {
	if ime.aiActive {
		return rimeState{}, nil, false
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok || strings.TrimSpace(state.Composition) == "" {
		return rimeState{}, nil, false
	}
	if len(ime.visibleCustomPhraseCandidatesForState(state)) > 0 {
		return rimeState{}, nil, false
	}
	snapshot, ok := ime.bertSnapshotForState(state)
	if !ok {
		return rimeState{}, nil, false
	}
	key := snapshot.Key
	if ime.bertCache == nil {
		return rimeState{}, nil, false
	}
	cached, ok := ime.bertCache.Get(key)
	if !ok || len(cached.Order) == 0 {
		return rimeState{}, nil, false
	}
	identity := identityBertRerankResult(len(state.Candidates)).Order
	if sameIntSlice(cached.Order, identity) {
		return rimeState{}, nil, false
	}
	return state, cached.Order, true
}

func sameIntSlice(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (ime *IME) isBertHandledKey(req *imecore.Request, candidateCount int) bool {
	if req == nil || candidateCount <= 0 {
		return false
	}
	if index, ok := ime.selectionKeyIndex(req); ok {
		return index < candidateCount
	}
	switch req.KeyCode {
	case vkUp, vkDown, vkSpace, vkReturn:
		return true
	}
	return false
}

func (ime *IME) handleBertKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	state, order, ok := ime.currentBertOverlay()
	if !ok || !ime.isBertHandledKey(req, min(len(order), len(state.Candidates))) {
		return false
	}
	ime.bertConsumeKeyUpCode = selectionShortcutConsumeCode(req)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleBertKeyUpFilter(req *imecore.Request, resp *imecore.Response) bool {
	if ime.bertConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.bertConsumeKeyUpCode {
		return false
	}
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleBertKeyDown(req *imecore.Request, resp *imecore.Response) bool {
	if ime.bertConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.bertConsumeKeyUpCode {
		return false
	}
	state, order, ok := ime.currentBertOverlay()
	if !ok {
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	}
	total := min(len(order), len(state.Candidates))
	switch req.KeyCode {
	case vkUp:
		if ime.bertCursor > 0 {
			ime.bertCursor--
		}
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	case vkDown:
		if ime.bertCursor < total-1 {
			ime.bertCursor++
		}
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	case vkSpace, vkReturn:
		if ime.commitBackendOverlayCandidate(resp, ime.bertCursor) {
			resp.ReturnValue = 1
			return true
		}
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	default:
		if index, ok := ime.selectionKeyIndex(req); ok && index < total {
			ime.bertCursor = index
			if ime.commitBackendOverlayCandidate(resp, index) {
				resp.ReturnValue = 1
				return true
			}
			ime.fillResponseFromCurrentState(resp)
			resp.ReturnValue = 1
			return true
		}
	}
	return false
}

func (ime *IME) handleBertKeyUp(req *imecore.Request, resp *imecore.Response) bool {
	if ime.bertConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.bertConsumeKeyUpCode {
		return false
	}
	ime.bertConsumeKeyUpCode = 0
	ime.fillResponseFromCurrentState(resp)
	resp.ReturnValue = 1
	return true
}
