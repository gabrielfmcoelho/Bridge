package api

import (
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/service"
)

// AutoresVarreduraStatus is the background GitLab-author scan: under way, or
// how the last one ended.
type AutoresVarreduraStatus = bgJobStatus[service.ResumoVarredura]

// autoresJob runs one scan at a time in the background.
type autoresJob = bgJob[service.ResumoVarredura]

var varreduraAutores = &autoresJob{}

const varreduraAutoresTimeout = 30 * time.Minute
