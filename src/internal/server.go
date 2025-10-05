package main

import (
    "encoding/json"
    "net/http"
    "strings"
)

var apiCatalog *Catalog
var apiStoreDir string

type apiEntry struct {
    BlobId   string   
    FileName string   
    Tags     []string 
}

func StartServer(addr, storeDir string, c *Catalog) error {
    apiCatalog = c
    apiStoreDir = storeDir

    mux := http.NewServeMux()
    mux.HandleFunc("/add", handleAdd)
    mux.HandleFunc("/add_tag", handleAddTag)
    mux.HandleFunc("/delete_tag", handleDeleteTag)
    mux.HandleFunc("/list", handleList)
    mux.HandleFunc("/show", handleShow)
    mux.HandleFunc("/delete", handleDelete)

    return http.ListenAndServe(addr, mux)
}

func handleAdd(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var req struct {
        Files []string 
        Tags  []string 
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if err := AddFiles(req.Files, req.Tags, apiStoreDir, apiCatalog); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json") 
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(map[string]any{"added": len(req.Files)})
}

func handleAddTag(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var req struct {
        Query []string 
        Tags  []string 
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if len(req.Tags) == 0 {
        http.Error(w, "missing tags to add", http.StatusBadRequest)
        return
    }

    matches := apiCatalog.FindByTags(req.Query)

    for _, f := range matches {
        for _, t := range req.Tags {
            apiCatalog.AddTag(f.BlobId, t)
        }
    }

    type respT struct {
        Updated int        
        Files   []apiEntry 
    }
    resp := respT{
        Updated: len(matches),
        Files:   make([]apiEntry, 0, len(matches)),
    }
    for _, f := range matches {
        resp.Files = append(resp.Files, apiEntry{
            BlobId:   f.BlobId,
            FileName: f.FileName,
            Tags:     apiCatalog.TagNamesOf(f),
        })
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}

func handleDeleteTag(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var req struct {
        Query []string 
        Tags  []string 
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    if len(req.Tags) == 0 {
        http.Error(w, "missing tags to remove", http.StatusBadRequest)
        return
    }

    matches := apiCatalog.FindByTags(req.Query)

    for _, f := range matches {
        for _, t := range req.Tags {
            apiCatalog.RemoveTag(f.BlobId, t)
        }
    }

    type respT struct {
        Updated int        
        Files   []apiEntry 
    }
    resp := respT{
        Updated: len(matches),
        Files:   make([]apiEntry, 0, len(matches)),
    }
    for _, f := range matches {
        resp.Files = append(resp.Files, apiEntry{
            BlobId:   f.BlobId,
            FileName: f.FileName,
            Tags:     apiCatalog.TagNamesOf(f),
        })
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}

func handleList(w http.ResponseWriter, r *http.Request) {
    tagsParam := r.URL.Query().Get("tags")
    if tagsParam == "" {
        http.Error(w, "missing tags", http.StatusBadRequest)
        return
    }
    tags := strings.Split(tagsParam, ",")
    files := apiCatalog.FindByTags(tags)

    resp := make([]apiEntry, 0, len(files))
    for _, f := range files {
        resp = append(resp, apiEntry{
            BlobId:   f.BlobId,
            FileName: f.FileName,
            Tags:     apiCatalog.TagNamesOf(f),
        })
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}

func handleShow(w http.ResponseWriter, r *http.Request) {
    all := apiCatalog.GetAllEntries()
    resp := make([]apiEntry, 0, len(all))
    for _, f := range all {
        resp = append(resp, apiEntry{
            BlobId:   f.BlobId,
            FileName: f.FileName,
            Tags:     apiCatalog.TagNamesOf(f),
        })
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodDelete {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    tagsParam := r.URL.Query().Get("tags")
    if tagsParam == "" {
        http.Error(w, "missing tags", http.StatusBadRequest)
        return
    }
    tags := strings.Split(tagsParam, ",")
    deleted, err := apiCatalog.DeleteByTags(tags, apiStoreDir)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    resp := make([]apiEntry, 0, len(deleted))
    for _, f := range deleted {
        resp = append(resp, apiEntry{
            BlobId:   f.BlobId,
            FileName: f.FileName,
            Tags:     apiCatalog.TagNamesOf(f),
        })
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}