package proto

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"google.golang.org/grpc/metadata"
)

const metadataKeySessionID = "session_id"

func WithSessionID(id uuid.UUID) metadata.MD {
	return metadata.New(map[string]string{
		metadataKeySessionID: id.String(),
	})
}

func FetchSessionID(md metadata.MD) (uuid.UUID, error) {
	id := md.Get(metadataKeySessionID)
	if len(id) != 1 {
		return uuid.UUID{}, fmt.Errorf("metadata has invalid number of session id keys: expected 1, found %d", len(id))
	}
	parsed, err := uuid.Parse(id[0])
	if err != nil {
		return uuid.UUID{}, errors.Wrap(err, "failed to parse session id")
	}
	return parsed, nil
}
