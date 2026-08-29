package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var driverFlag string

var uploadCmd = &cobra.Command{
	Use:   "upload [file path]",
	Short: "Upload a file into the DAM library",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return err
		}

		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		mimeType := http.DetectContentType(buf[:n])
		_, _ = file.Seek(0, 0)

		asset, err := assetService.Upload(
			context.Background(),
			driverFlag,
			filepath.Base(filePath),
			mimeType,
			info.Size(),
			file,
		)
		if err != nil {
			return err
		}

		fmt.Printf("✅ Uploaded %s [ID: %s, Driver: %s] (%d bytes)\n", asset.Filename, asset.ID, asset.StorageDriver, asset.ByteSize)
		return nil
	},
}

func init() {
	uploadCmd.Flags().StringVarP(&driverFlag, "driver", "d", "local", "Storage driver to use (e.g. local, sftp_tenant1)")
	rootCmd.AddCommand(uploadCmd)
}
