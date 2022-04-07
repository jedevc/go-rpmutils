package rpmutils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestUncompress(t *testing.T) {
	defer goleak.VerifyNone(t)
	// built with e.g.: rpmbuild -bb payload-test.spec -D'_binary_payload w.ufdio'
	payloadTypes := []string{"w3.zstdio", "w6.lzdio", "w6.xzdio", "w9.bzdio", "w9.gzdio", "w.ufdio"}
	for _, payloadType := range payloadTypes {
		t.Run(payloadType, func(t *testing.T) {
			// open rpm
			fp := filepath.Join("testdata", "payload-test-0.1-"+payloadType+".x86_64.rpm")
			f, err := os.Open(fp)
			require.NoError(t, err)
			defer f.Close()
			rpm, err := ReadRpm(f)
			require.NoError(t, err)
			// consume payload
			var files int
			payload, err := rpm.PayloadReaderExtended()
			require.NoError(t, err)
			for {
				_, err := payload.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				_, err = io.Copy(ioutil.Discard, payload)
				require.NoError(t, err)
				files++
			}
			assert.Equal(t, 1, files)
		})
	}
}

func TestUncompressEmpty(t *testing.T) {
	f, err := os.Open("testdata/empty-0.1-1.x86_64.rpm")
	require.NoError(t, err)
	defer f.Close()
	rpm, err := ReadRpm(f)
	require.NoError(t, err)
	payload, err := rpm.PayloadReaderExtended()
	require.NoError(t, err)
	_, err = payload.Next()
	assert.ErrorIs(t, err, io.EOF)
}

func TestUncompressNonContinuousLinkGroups(t *testing.T) {
	f, err := os.Open("testdata/crypto-policies-20210617-1.gitc776d3e.el8.noarch.rpm")
	require.NoError(t, err)
	defer f.Close()
	rpm, err := ReadRpm(f)
	require.NoError(t, err)
	digestAlgo, err := rpm.Header.GetInts(FILEDIGESTALGO)
	require.NoError(t, err)
	require.Equal(t, []int{PGPHASHALGO_SHA256}, digestAlgo)

	payload, err := rpm.PayloadReaderExtended()
	require.NoError(t, err)

	for {
		file, err := payload.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		data, err := io.ReadAll(payload)
		require.NoError(t, err)

		if payload.IsLink() {
			require.Equal(t, 0, len(data), "wrong size read for linked file %q", file.Name())
			continue
		}

		require.Equal(t, int(file.Size()), len(data), "wrong size read for regular file %q", file.Name())
		if file.Size() > 0 {
			checksum := sha256.Sum256(data)
			require.Equal(t, file.Digest(), hex.EncodeToString(checksum[:]), "checksums did not match for file %q", file.Name())
		}
	}
}
