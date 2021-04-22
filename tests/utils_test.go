package tests

import (
	"os"
	"testing"

	oss "github.com/aos-dev/go-service-oss"
	ps "github.com/aos-dev/go-storage/v3/pairs"
	"github.com/aos-dev/go-storage/v3/types"
	"github.com/google/uuid"
)

func setupTest(t *testing.T) types.Storager {
	t.Log("Setup test for oss")

	store, err := oss.NewStorager(
		ps.WithCredential(os.Getenv("STORAGE_OSS_CREDENTIAL")),
		ps.WithName(os.Getenv("STORAGE_OSS_NAME")),
		ps.WithEndpoint(os.Getenv("STORAGE_OSS_ENDPOINT")),
		ps.WithWorkDir("/"+uuid.New().String()+"/"),
	)
	if err != nil {
		t.Errorf("new storager: %v", err)
	}
	return store
}
