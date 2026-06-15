// Package migrations embeds the SQL migration files into the binary so the
// application can run them at startup without shipping the .sql files alongside
// the executable. golang-migrate reads them through the iofs source driver.
package migrations

import "embed"

// FS holds every *.sql file in this directory. The //go:embed directive is
// evaluated at compile time; if a file is missing the build fails — you can't
// accidentally deploy a binary with stale migrations.
//
//go:embed *.sql
var FS embed.FS
