//go:build !unix

package config

import "io/fs"

func checkTrust(string, fs.FileInfo) error {
	return nil
}
