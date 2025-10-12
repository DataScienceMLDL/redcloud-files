package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/dgraph-io/badger/v4"
	"github.com/sekai02/redcloud-files/internal/api"
	"github.com/sekai02/redcloud-files/internal/device"
	"github.com/sekai02/redcloud-files/internal/ids"
	"github.com/sekai02/redcloud-files/internal/inode"
	"github.com/sekai02/redcloud-files/internal/persistence"
	"github.com/sekai02/redcloud-files/internal/scope"
	"github.com/sekai02/redcloud-files/internal/storage"
	"github.com/sekai02/redcloud-files/internal/tag/taglist"
	"github.com/sekai02/redcloud-files/internal/tag/tagtree"
)

var service *api.Service
var store storage.Store

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	idGen := ids.NewGenerator()

	badgerStore, err := storage.NewBadgerStore("./data/badger")
	if err != nil {
		log.Fatal("Failed to open BadgerDB:", err)
	}
	store = badgerStore
	defer store.Close()

	deviceMgr := device.NewManager(idGen)
	inodeMgr := inode.NewManager(store, idGen)
	tagList := taglist.NewList(idGen)
	tagTree := tagtree.NewTree()
	scopeMgr := scope.NewManager(idGen, tagList)

	data, err := store.LoadMetadata("system")
	if err != nil {
		if err == badger.ErrKeyNotFound {
			slog.Info("No existing metadata found, starting fresh")
			deviceMgr.RegisterDevice("default")
		} else {
			slog.Warn("Failed to load metadata", "error", err)
			deviceMgr.RegisterDevice("default")
		}
	} else {
		slog.Info("Loading existing metadata from storage")
		err = persistence.LoadMetadata(data, idGen, deviceMgr, inodeMgr, tagList, tagTree)
		if err != nil {
			log.Fatal("Failed to restore metadata:", err)
		}
		slog.Info("Metadata restored successfully")
	}

	service = api.NewService(deviceMgr, inodeMgr, tagList, tagTree, scopeMgr, idGen, store)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Shutting down gracefully...")
		store.Close()
		os.Exit(0)
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/files", handleFiles)
	mux.HandleFunc("/v1/files/", handleFileOps)
	mux.HandleFunc("/v1/tags/", handleTags)
	mux.HandleFunc("/v1/devices", handleDevices)
	mux.HandleFunc("/v1/scopes", handleScopes)
	mux.HandleFunc("/v1/scopes/", handleScopeOps)
	mux.HandleFunc("/v1/import", handleImport)
	mux.HandleFunc("/v1/export/", handleExport)

	addr := ":8080"
	slog.Info("Starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID uint64 `json:"device_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fid, err := service.Create(context.Background(), req.DeviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]uint64{"file_id": fid})
}

func handleFileOps(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/files/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	dev, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid device ID", http.StatusBadRequest)
		return
	}

	fid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		http.Error(w, "invalid file ID", http.StatusBadRequest)
		return
	}

	if len(parts) == 3 && parts[2] == "copy" {
		handleCopy(w, r, dev, fid)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		handleDelete(w, r, dev, fid)
	case http.MethodGet:
		handleRead(w, r, dev, fid)
	case http.MethodPut:
		handleWrite(w, r, dev, fid)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleCopy(w http.ResponseWriter, r *http.Request, dev, fid uint64) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dstStr := r.URL.Query().Get("dst")
	if dstStr == "" {
		http.Error(w, "missing destination device", http.StatusBadRequest)
		return
	}

	dst, err := strconv.ParseUint(dstStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid destination device", http.StatusBadRequest)
		return
	}

	newFID, err := service.Copy(context.Background(), dev, fid, dst)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]uint64{"file_id": newFID})
}

func handleDelete(w http.ResponseWriter, r *http.Request, dev, fid uint64) {
	err := service.Delete(context.Background(), dev, fid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleRead(w http.ResponseWriter, r *http.Request, dev, fid uint64) {
	offStr := r.URL.Query().Get("off")
	lenStr := r.URL.Query().Get("len")

	off := int64(0)
	length := int64(1 << 30)

	if offStr != "" {
		var err error
		off, err = strconv.ParseInt(offStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
	}

	if lenStr != "" {
		var err error
		length, err = strconv.ParseInt(lenStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid length", http.StatusBadRequest)
			return
		}
	}

	data, err := service.Read(context.Background(), dev, fid, off, length)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

func handleWrite(w http.ResponseWriter, r *http.Request, dev, fid uint64) {
	offStr := r.URL.Query().Get("off")
	off := int64(0)

	if offStr != "" {
		var err error
		off, err = strconv.ParseInt(offStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	n, err := service.Write(context.Background(), dev, fid, off, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"written": n})
}

func handleTags(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/tags/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	dev, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid device ID", http.StatusBadRequest)
		return
	}

	fid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		http.Error(w, "invalid file ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		handleTagAdd(w, r, dev, fid)
	case http.MethodDelete:
		handleTagRemove(w, r, dev, fid)
	case http.MethodGet:
		handleTagList(w, r, dev, fid)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleTagAdd(w http.ResponseWriter, r *http.Request, dev, fid uint64) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := service.TagAdd(context.Background(), dev, fid, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleTagRemove(w http.ResponseWriter, r *http.Request, dev, fid uint64) {
	tagName := r.URL.Query().Get("name")
	if tagName == "" {
		http.Error(w, "missing tag name", http.StatusBadRequest)
		return
	}

	err := service.TagRemove(context.Background(), dev, fid, tagName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleTagList(w http.ResponseWriter, r *http.Request, dev, fid uint64) {
	tags, err := service.TagList(context.Background(), dev, fid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"tags": tags})
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := service.DeviceList(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]uint64{"devices": devices})
}

func handleScopes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sid, err := service.MkScope(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]uint64{"scope_id": sid})
}

func handleScopeOps(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/scopes/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	sid, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid scope ID", http.StatusBadRequest)
		return
	}

	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	operation := parts[1]

	switch operation {
	case "sources":
		handleSources(w, r, sid)
	case "filters":
		handleFilters(w, r, sid)
	case "list":
		handleScopeList(w, r, sid)
	default:
		http.Error(w, "unknown operation", http.StatusBadRequest)
	}
}

func handleSources(w http.ResponseWriter, r *http.Request, sid uint64) {
	var req struct {
		SourceID uint64 `json:"source_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		err := service.ScopeAddSource(context.Background(), sid, req.SourceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		err := service.ScopeRmSource(context.Background(), sid, req.SourceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleFilters(w http.ResponseWriter, r *http.Request, sid uint64) {
	var req struct {
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		err := service.ScopeAddFilter(context.Background(), sid, req.Tags...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		err := service.ScopeRmFilter(context.Background(), sid, req.Tags...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleScopeList(w http.ResponseWriter, r *http.Request, sid uint64) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results, err := service.List(context.Background(), sid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type filePair struct {
		DeviceID uint64 `json:"device_id"`
		FileID   uint64 `json:"file_id"`
	}

	response := make([]filePair, len(results))
	for i, pair := range results {
		response[i] = filePair{
			DeviceID: pair[0],
			FileID:   pair[1],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"files": response})
}

func handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID uint64   `json:"device_id"`
		Path     string   `json:"path"`
		Tags     []string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fid, filename, err := service.ImportFile(context.Background(), req.DeviceID, req.Path, req.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_id":  fid,
		"filename": filename,
	})
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/export/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	dev, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid device ID", http.StatusBadRequest)
		return
	}

	fid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		http.Error(w, "invalid file ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := service.ExportFile(context.Background(), dev, fid, req.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
