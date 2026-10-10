package api

import (
	"watchAlert/internal/services"
	"watchAlert/internal/types"
	"watchAlert/pkg/response"

	"github.com/gin-gonic/gin"
)

type migrationController struct{}

var MigrationController = new(migrationController)

// ExportConfig GET /api/watchalert/export/config
func (migrationController) ExportConfig(ctx *gin.Context) {
	data, err := services.MigrationService.Export()
	response.Envelope(ctx, data, err)
}

// ImportConfig POST /api/watchalert/import/config
func (migrationController) ImportConfig(ctx *gin.Context) {
	r := new(types.ImportConfigReq)
	BindJson(ctx, r)
	if ctx.IsAborted() {
		return
	}
	data, err := services.MigrationService.Import(r)
	response.Envelope(ctx, data, err)
}
