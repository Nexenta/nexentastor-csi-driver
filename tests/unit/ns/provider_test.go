package provider_test

import (
	"testing"

	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

func TestProvider_Filesystem(t *testing.T) {
	path := "/pool/dataset/fs"
	expectedShareName := "pool_dataset_fs"

	fs := ns.Filesystem{Path: path}

	t.Run("Filesystem.GetDefaultSmbShareName() should return default SMB share name", func(t *testing.T) {
		shareName := fs.GetDefaultSmbShareName()
		if shareName != expectedShareName {
			t.Errorf("expected '%s', but got '%s' instead", expectedShareName, shareName)
		}
	})
}
