package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/selfupdate"
	"github.com/peregrine-digital/activate-framework/cli/transport"
)

// Daemon is the JSON-RPC server that dispatches requests to an ActivateService.
type Daemon struct {
	service   ActivateAPI
	transport *transport.Transport
	version   string
}

// NewDaemon creates a daemon wired to the given service and transport.
func NewDaemon(service ActivateAPI, t *transport.Transport, version string) *Daemon {
	return &Daemon{service: service, transport: t, version: version}
}

// Serve reads requests from the transport and dispatches them until EOF or shutdown.
func (d *Daemon) Serve() error {
	for {
		req, err := d.transport.ReadMessage()
		if err != nil {
			if err == io.EOF || isClosedPipe(err) {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		resp := d.dispatch(req)
		if err := d.transport.WriteResponse(resp); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		if isMutating(req.Method) {
			_ = d.transport.WriteNotification(transport.StateChangedNotification())
		}
	}
}

func (d *Daemon) dispatch(req *transport.Request) *transport.Response {
	switch req.Method {
	case transport.MethodInitialize:
		return d.handleInitialize(req)
	case transport.MethodShutdown:
		return transport.SuccessResponse(req.ID, map[string]bool{"ok": true})
	case transport.MethodStateGet:
		return d.handleStateGet(req)
	case transport.MethodConfigGet:
		return d.handleConfigGet(req)
	case transport.MethodConfigSet:
		return d.handleConfigSet(req)
	case transport.MethodManifestList:
		return d.handleManifestList(req)
	case transport.MethodManifestFiles:
		return d.handleManifestFiles(req)
	case transport.MethodRepoAdd:
		return d.handleRepoAdd(req)
	case transport.MethodRepoRemove:
		return d.handleRepoRemove(req)
	case transport.MethodSync:
		return d.handleSync(req)
	case transport.MethodUpdate:
		return d.handleUpdate(req)
	case transport.MethodFileInstall:
		return d.handleFileInstall(req)
	case transport.MethodFileUninstall:
		return d.handleFileUninstall(req)
	case transport.MethodFileDiff:
		return d.handleFileDiff(req)
	case transport.MethodFileSkip:
		return d.handleFileSkip(req)
	case transport.MethodFileOverride:
		return d.handleFileOverride(req)
	case transport.MethodTelemetryRun:
		return d.handleTelemetryRun(req)
	case transport.MethodTelemetryLog:
		return d.handleTelemetryLog(req)
	case transport.MethodCheckUpdate:
		return d.handleCheckUpdate(req)
	case transport.MethodSelfUpdate:
		return d.handleSelfUpdate(req)
	default:
		return transport.ErrorResponse(req.ID, transport.ErrCodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func isMutating(method string) bool {
	switch method {
	case transport.MethodConfigSet, transport.MethodRepoAdd, transport.MethodRepoRemove,
		transport.MethodSync, transport.MethodUpdate, transport.MethodFileInstall,
		transport.MethodFileUninstall, transport.MethodFileSkip, transport.MethodFileOverride,
		transport.MethodTelemetryRun:
		return true
	}
	return false
}

func isClosedPipe(err error) bool {
	return err != nil && (errors.Is(err, io.ErrClosedPipe) ||
		err.Error() == "io: read/write on closed pipe")
}

// ── Handlers ───────────────────────────────────────────────────

func (d *Daemon) handleInitialize(req *transport.Request) *transport.Response {
	var params transport.InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
		}
	}
	if params.ProjectDir != "" {
		d.service.Initialize(params.ProjectDir)
	}

	return transport.SuccessResponse(req.ID, transport.InitializeResult{
		Version: d.version,
		Capabilities: []string{
			"state", "config", "manifests", "files",
			"repo", "sync", "update", "diff",
			"telemetry", "overrides", "selfUpdate",
		},
	})
}

func (d *Daemon) handleStateGet(req *transport.Request) *transport.Response {
	return transport.SuccessResponse(req.ID, d.service.GetState())
}

func (d *Daemon) handleConfigGet(req *transport.Request) *transport.Response {
	var params transport.ConfigGetParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
		}
	}
	cfg, err := d.service.GetConfig(params.Scope)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
	}
	return transport.SuccessResponse(req.ID, cfg)
}

func (d *Daemon) handleConfigSet(req *transport.Request) *transport.Response {
	var params transport.ConfigSetParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
		}
	}

	updates := params.Updates
	if updates == nil {
		updates = &model.Config{}
	}
	if params.Manifest != "" {
		updates.Manifest = params.Manifest
	}
	if params.Tier != "" {
		updates.Tier = params.Tier
	}
	if params.TelemetryEnabled != nil {
		updates.TelemetryEnabled = params.TelemetryEnabled
	}

	result, err := d.service.SetConfig(params.Scope, updates)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleManifestList(req *transport.Request) *transport.Response {
	return transport.SuccessResponse(req.ID, d.service.ListManifests())
}

func (d *Daemon) handleManifestFiles(req *transport.Request) *transport.Response {
	var params transport.ManifestFilesParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
		}
	}
	result, err := d.service.ListFiles(params.Manifest, params.Tier, params.Category)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleRepoAdd(req *transport.Request) *transport.Response {
	result, err := d.service.RepoAdd()
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleRepoRemove(req *transport.Request) *transport.Response {
	if err := d.service.RepoRemove(); err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, map[string]bool{"ok": true})
}

func (d *Daemon) handleSync(req *transport.Request) *transport.Response {
	result, err := d.service.Sync()
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleUpdate(req *transport.Request) *transport.Response {
	result, err := d.service.Update()
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileInstall(req *transport.Request) *transport.Response {
	var params transport.FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.InstallFile(params.File)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileUninstall(req *transport.Request) *transport.Response {
	var params transport.FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.UninstallFile(params.File)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileDiff(req *transport.Request) *transport.Response {
	var params transport.FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.DiffFile(params.File)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileSkip(req *transport.Request) *transport.Response {
	var params transport.FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.SkipUpdate(params.File)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileOverride(req *transport.Request) *transport.Response {
	var params transport.FileOverrideParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.SetOverride(params.File, params.Override)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleTelemetryRun(req *transport.Request) *transport.Response {
	var params transport.TelemetryRunParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return transport.ErrorResponse(req.ID, transport.ErrCodeInvalidParams, "invalid telemetryRun params")
		}
	}
	result, err := d.service.RunTelemetry(params.Token)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}

func (d *Daemon) handleTelemetryLog(req *transport.Request) *transport.Response {
	entries, err := d.service.ReadTelemetryLog()
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, entries)
}

func (d *Daemon) handleCheckUpdate(req *transport.Request) *transport.Response {
	var params transport.CheckUpdateParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	var entry *selfupdate.CacheEntry
	if params.Force {
		entry = selfupdate.CheckLive(d.version, params.ExtensionVersion, params.Token)
	} else {
		entry = selfupdate.CheckCached(d.version, params.ExtensionVersion, params.Token)
	}
	if entry == nil {
		return transport.SuccessResponse(req.ID, selfupdate.CacheEntry{})
	}
	return transport.SuccessResponse(req.ID, entry)
}

func (d *Daemon) handleSelfUpdate(req *transport.Request) *transport.Response {
	var params transport.CheckUpdateParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	result, err := selfupdate.Run(d.version, params.Token)
	if err != nil {
		return transport.ErrorResponse(req.ID, transport.ErrCodeInternal, err.Error())
	}
	return transport.SuccessResponse(req.ID, result)
}
