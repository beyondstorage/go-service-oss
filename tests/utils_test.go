package tests

import (
	"os"
	"testing"

	oss "github.com/beyondstorage/go-service-oss/v2"
	ps "github.com/beyondstorage/go-storage/v4/pairs"
	"github.com/beyondstorage/go-storage/v4/types"
	"github.com/google/uuid"
)

func setupTest(t *testing.T) types.Storager {
	t.Log("Setup test for oss")

	store, err := oss.NewStorager(
		ps.WithCredential(os.Getenv("STORAGE_OSS_CREDENTIAL")),
		ps.WithName(os.Getenv("STORAGE_OSS_NAME")),
		ps.WithEndpoint(os.Getenv("STORAGE_OSS_ENDPOINT")),
		ps.WithWorkDir("/"+uuid.New().String()+"/"),
		oss.WithStorageFeatures(oss.StorageFeatures{
			VirtualDir: true,
		}),
	)
	if err != nil {
		t.Errorf("new storager: %v", err)
	}
	return store
}
