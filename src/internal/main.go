package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Por ahora solo se guardan en memoria por lo q si ejecutas el programa y luego lo cierras
// se pierde todo lo que tenias en el catalogo aunque en /store quedan los blobs
// TODO : Persistir el catalogo en un archivo y cargarlo al iniciar el programa (es decir sqlLite o
//  un json simple (agregar indices Invertidos para busquedas rapidas))

// main initializes the TBFS (Tag Based File System) CLI application.
// It creates the storage directory, initializes the file catalog, and enters a command loop
// where users can interactively add files with tags, list files by tags, show the catalog contents,
// and delete files by tags. Supported commands are:
//   - add file1 file2 ... --tags tag1,tag2 : Adds files with associated tags to the catalog.
//   - list tag1,tag2                      : Lists files matching the specified tags.
//   - show                                : Displays all entries in the catalog.
//   - delete tag1,tag2                    : Deletes files matching the specified tags from the catalog and storage.
//   - exit                                : Exits the application.


const apiBase = "http://127.0.0.1:8080"

func main() {
    // Server mode: `go run . serve`
    if len(os.Args) > 1 && os.Args[1] == "serve" {
        storeDir := "./store"
        _ = os.MkdirAll(storeDir, 0755)
        c := NewCatalog()
        fmt.Println("API escuchando en", apiBase)
        if err := StartServer(":8080", storeDir, c); err != nil {
            fmt.Println("Error al iniciar API:", err)
        }
        return
    }

    // Interactive CLI mode
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Println("Bienvenido a TBFS (CLI sobre API). Ejecuta `go run . serve` en otra terminal primero.")
    fmt.Println("Comandos: add file1 file2 ... --tags t1,t2 | list t1,t2 | show | delete t1,t2 | exit")

    for {
        fmt.Print("tbfs> ")
        if !scanner.Scan() {
            break
        }
        line := scanner.Text()
        if line == "exit" {
            fmt.Println("Exiting...")
            break
        }
        args := strings.Fields(line)
        if len(args) == 0 {
            continue
        }
        switch args[0] {
        case "add":
            if len(args) < 2 {
                fmt.Println("Usage: add file1 file2 ... --tags tag1,tag2")
                continue
            }
            var fileList []string
            var tags []string
            tagsIndex := -1
            for i, a := range args {
                if a == "--tags" {
                    tagsIndex = i
                    break
                }
            }
            if tagsIndex == -1 {
                fileList = args[1:]
            } else {
                fileList = args[1:tagsIndex]
                if tagsIndex+1 < len(args) {
                    tags = strings.Split(args[tagsIndex+1], ",")
                }
            }
            body, _ := json.Marshal(map[string]any{
                "files": fileList,
                "tags":  tags,
            })
            resp, err := http.Post(apiBase+"/add", "application/json", bytes.NewBuffer(body))
            if err != nil {
                fmt.Println("Error:", err)
                continue
            }
            resp.Body.Close()
            fmt.Println("Archivos enviados a la API.")

        case "list":
            if len(args) < 2 {
                fmt.Println("Usage: list tag1,tag2")
                continue
            }
            q := url.Values{}
            q.Set("tags", args[1])
            resp, err := http.Get(apiBase + "/list?" + q.Encode())
            if err != nil {
                fmt.Println("Error:", err)
                continue
            }
            var entries []apiEntry
            if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
                fmt.Println("Error:", err)
            }
            resp.Body.Close()
            fmt.Printf("Results found: %d\n", len(entries))
            for _, e := range entries {
                fmt.Printf("Blob: %s | File: %s | Tags: %v\n", e.BlobId, e.FileName, e.Tags)
            }

        case "show":
            resp, err := http.Get(apiBase + "/show")
            if err != nil {
                fmt.Println("Error:", err)
                continue
            }
            var entries []apiEntry
            if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
                fmt.Println("Error:", err)
            }
            resp.Body.Close()
            fmt.Printf("Total entries: %d\n", len(entries))
            for _, e := range entries {
                fmt.Printf("BlobID: %s | File: %s | Tags: %v\n", e.BlobId, e.FileName, e.Tags)
            }

        case "delete":
            if len(args) < 2 {
                fmt.Println("Usage: delete tag1,tag2")
                continue
            }
            req, _ := http.NewRequest(http.MethodDelete, apiBase+"/delete?tags="+url.QueryEscape(args[1]), nil)
            resp, err := http.DefaultClient.Do(req)
            if err != nil {
                fmt.Println("Error:", err)
                continue
            }
            var entries []apiEntry
            if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
                fmt.Println("Error:", err)
            }
            resp.Body.Close()
            for _, e := range entries {
                fmt.Printf("Deleted Blob: %s | File: %s | Tags: %v\n", e.BlobId, e.FileName, e.Tags)
            }

        default:
            fmt.Println("Unknown command:", args[0])
        }
    }
}