// Package corpus anchors the pinned OSS evaluation corpus in go.mod.
//
// The blank imports keep `go mod tidy` from dropping the pinned requires; the
// module proxy (checksummed by go.sum) is the delivery mechanism for the
// corpus source, so no git clone — and no mutable ref — is involved. The
// accuracy harness (scripts/evaluate-oss.sh) runs `gss evaluate` from this
// directory with import-path patterns, which loads exactly the pinned release
// source out of the module cache.
package corpus

import (
	_ "github.com/gin-gonic/gin"
	_ "github.com/sirupsen/logrus"
	_ "github.com/spf13/cobra"
	_ "github.com/valyala/fasthttp"
	_ "go.etcd.io/bbolt"
	_ "go.uber.org/zap"
)
