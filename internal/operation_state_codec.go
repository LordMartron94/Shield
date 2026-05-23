package internal

import (
	"encoding/json"
	"fmt"
)

/*
OperationStateCodec serializes operation state across subprocess guard boundaries.

Snapshots must contain only process-portable logical fields (versioned JSON recommended).
*/
type OperationStateCodec[TState any] struct {
	Serialize   func(state TState) ([]byte, error)
	Deserialize func(snapshot []byte) (TState, error)
}

type operationStateCodecBox struct {
	stateful    bool
	serialize   func(state any) ([]byte, error)
	deserialize func(snapshot []byte) (state any, err error)
}

func operationStateCodecBoxFrom[TState any](codec *OperationStateCodec[TState]) operationStateCodecBox {
	if codec == nil {
		var zero TState
		if _, isStateless := any(zero).(struct{}); isStateless {
			return operationStateCodecBoxStructEmpty()
		}
		return operationStateCodecBox{stateful: true}
	}
	return operationStateCodecBox{
		stateful: true,
		serialize: func(state any) ([]byte, error) {
			return codec.Serialize(state.(TState))
		},
		deserialize: func(snapshot []byte) (any, error) {
			return codec.Deserialize(snapshot)
		},
	}
}

func operationStateCodecBoxStructEmpty() operationStateCodecBox {
	return operationStateCodecBox{
		stateful: false,
		serialize: func(state any) ([]byte, error) {
			_ = state
			return []byte("{}"), nil
		},
		deserialize: func(snapshot []byte) (any, error) {
			if len(snapshot) == 0 {
				return struct{}{}, nil
			}
			var payload struct{}
			if err := json.Unmarshal(snapshot, &payload); err != nil {
				return struct{}{}, err
			}
			return struct{}{}, nil
		},
	}
}

func operationStateCodecBoxSerialize(box operationStateCodecBox, state any) ([]byte, error) {
	if box.serialize == nil {
		return nil, fmt.Errorf("operation state codec: serialize is nil")
	}
	return box.serialize(state)
}

func operationStateCodecBoxDeserialize(box operationStateCodecBox, snapshot []byte) (any, error) {
	if box.deserialize == nil {
		return nil, fmt.Errorf("operation state codec: deserialize is nil")
	}
	return box.deserialize(snapshot)
}

func operationStateCodecBoxRequiresCodecForSubprocess(box operationStateCodecBox, operationName string) error {
	if !box.stateful {
		return nil
	}
	if box.serialize == nil || box.deserialize == nil {
		return fmt.Errorf(
			"stateful operation %q requires OperationStateCodec for subprocess guard isolation",
			operationName,
		)
	}
	return nil
}
