package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Daemon is the JSON-RPC server that dispatches requests to an ActivateService.
type Daemon struct {
	service   ActivateAPI
	transport *Transport
}

// NewDaemon creates a daemon wired to the given service and transport.
func NewDaemon(service ActivateAPI, transport *Transport) *Daemon {
	return &Daemon{service: service, transport: transport}
}

// Serve reads requests from the transport and dispatches them until EOF or shutdown.
func (d *Daemon) Serve() error {
	for {
		req, err := d.transport.ReadMessage()
		if err != nil {
			if err == io.EOF || isClosedPipe(err) {
				return nil // clean shutdown
			}
			return fmt.Errorf("read: %w", err)
		}

		resp := d.dispatch(req)
		if err := d.transport.WriteResponse(resp); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		// Emit state-changed notification after mutating operations
		if isMutating(req.Method) {
			_ = d.transport.WriteNotification(StateChangedNotification())
		}
	}
}

// dispatch routes a request to the appropriate handler.
func (d *Daemon) dispatch(req *Request) *Response {
	switch req.Method {
	case MethodInitialize:
		return d.handleInitialize(req)
	case MethodShutdown:
		return SuccessResponse(req.ID, map[string]bool{"ok": true})
	case MethodStateGet:
		return d.handleStateGet(req)
	case MethodConfigGet:
		return d.handleConfigGet(req)
	case MethodConfigSet:
		return d.handleConfigSet(req)
	case MethodManifestList:
		return d.handleManifestList(req)
	case MethodManifestFiles:
		return d.handleManifestFiles(req)
	case MethodRepoAdd:
		return d.handleRepoAdd(req)
	case MethodRepoRemove:
		return d.handleRepoRemove(req)
	case MethodSync:
		return d.handleSync(req)
	case MethodUpdate:
		return d.handleUpdate(req)
	case MethodFileInstall:
		return d.handleFileInstall(req)
	case MethodFileUninstall:
		return d.handleFileUninstall(req)
	case MethodFileDiff:
		return d.handleFileDiff(req)
	case MethodFileSkip:
		return d.handleFileSkip(req)
	case MethodFileOverride:
		return d.handleFileOverride(req)
	case MethodTelemetryRun:
		return d.handleTelemetryRun(req)
	case MethodTelemetryLog:
		return d.handleTelemetryLog(req)
	default:
		return ErrorResponse(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

// isMutating returns true for methods that change state on disk.
func isMutating(method string) bool {
	switch method {
	case MethodConfigSet, MethodRepoAdd, MethodRepoRemove, MethodSync, MethodUpdate,
		MethodFileInstall, MethodFileUninstall, MethodFileSkip, MethodFileOverride,
		MethodTelemetryRun:
		return true
	}
	return false
}

func isClosedPipe(err error) bool {
	return err != nil && (errors.Is(err, io.ErrClosedPipe) ||
		err.Error() == "io: read/write on closed pipe")
}

// ── Handlers ───────────────────────────────────────────────────

func (d *Daemon) handleInitialize(req *Request) *Response {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
		}
	}
	if params.ProjectDir != "" {
		d.service.Initialize(params.ProjectDir)
	}

	return SuccessResponse(req.ID, InitializeResult{
		Version: version,
		Capabilities: []string{
			"state", "config", "manifests", "files",
			"repo", "sync", "update", "diff",
			"telemetry", "overrides",
		},
	})
}

func (d *Daemon) handleStateGet(req *Request) *Response {
	return SuccessResponse(req.ID, d.service.GetState())
}

func (d *Daemon) handleConfigGet(req *Request) *Response {
	var params ConfigGetParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
		}
	}
	cfg, err := d.service.GetConfig(params.Scope)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	return SuccessResponse(req.ID, cfg)
}

func (d *Daemon) handleConfigSet(req *Request) *Response {
	var params ConfigSetParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
		}
	}

	updates := params.Updates
	if updates == nil {
		updates = &Config{}
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
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleManifestList(req *Request) *Response {
	return SuccessResponse(req.ID, d.service.ListManifests())
}

func (d *Daemon) handleManifestFiles(req *Request) *Response {
	var params ManifestFilesParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
		}
	}
	result, err := d.service.ListFiles(params.Manifest, params.Tier, params.Category)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleRepoAdd(req *Request) *Response {
	result, err := d.service.RepoAdd()
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleRepoRemove(req *Request) *Response {
	if err := d.service.RepoRemove(); err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, map[string]bool{"ok": true})
}

func (d *Daemon) handleSync(req *Request) *Response {
	result, err := d.service.Sync()
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleUpdate(req *Request) *Response {
	result, err := d.service.Update()
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileInstall(req *Request) *Response {
	var params FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.InstallFile(params.File)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileUninstall(req *Request) *Response {
	var params FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.UninstallFile(params.File)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileDiff(req *Request) *Response {
	var params FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.DiffFile(params.File)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileSkip(req *Request) *Response {
	var params FileParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.SkipUpdate(params.File)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleFileOverride(req *Request) *Response {
	var params FileOverrideParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, err.Error())
	}
	result, err := d.service.SetOverride(params.File, params.Override)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleTelemetryRun(req *Request) *Response {
	var params TelemetryRunParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return ErrorResponse(req.ID, ErrCodeInvalidParams, "invalid telemetryRun params")
		}
	}
	result, err := d.service.RunTelemetry(params.Token)
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, result)
}

func (d *Daemon) handleTelemetryLog(req *Request) *Response {
	entries, err := d.service.ReadTelemetryLog()
	if err != nil {
		return ErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return SuccessResponse(req.ID, entries)
}
