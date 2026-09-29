package web

import "embed"

// Assets embeds all HTML, CSS, and JS web dashboard files.
//
//go:embed index.html css/* js/*
var Assets embed.FS
