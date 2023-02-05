package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	logger      = log.Default()
	uploadMutex sync.Map
)

const (
	DataDir                       = "/tmp"
	MaxWriteBufferSizeBytes int64 = 64 * 1024
	MaxReadBufferSizeBytes  int64 = 1024 * 1024
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Println(r.Method, r.URL.Path)

		setCorsHeaders(w)

		switch r.Method {
		case http.MethodGet:
			getHandler(w, r)
		case http.MethodOptions:
			optionsHandler(w, r)
		case http.MethodDelete:
			deleteHandler(w, r)
		case http.MethodPost:
			postHandler(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		setCorsHeaders(w)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		now := time.Now().Format(time.RFC3339Nano)
		w.Write([]byte(now))
	})

	logger.Println("Server start")
	logger.Fatal(http.ListenAndServe(":8001", nil))
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	_, uploadingInProgress := uploadMutex.LoadOrStore(uri, true)
	if uploadingInProgress {
		w.WriteHeader(http.StatusConflict)
		return
	}
	defer func() {
		uploadMutex.Delete(uri)
	}()

	if err := processFileUpload(r); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func processFileUpload(r *http.Request) error {
	filePath := getFullFilePath(r.URL.Path)

	file, openFileErr := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	if openFileErr != nil {
		logger.Println("POST openfile err", filePath, openFileErr.Error())
		return openFileErr
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			logger.Println("POST closefile err", filePath, err.Error())
		}
	}(file)

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			logger.Println("POST closebody err", filePath, err.Error())
		}
	}(r.Body)

	var postBytesTotal int
	for {
		buffer := make([]byte, MaxWriteBufferSizeBytes)
		postBytes, readErr := r.Body.Read(buffer)

		if postBytes != 0 {
			postBytesTotal += postBytes
			_, writeErr := file.Write(buffer[:postBytes])
			if writeErr != nil {
				logger.Println("Write file error", filePath, writeErr)
				return writeErr
			}
		}

		switch readErr {
		case nil:
			continue
		case io.EOF:
			logger.Println("POST EOF", filePath, postBytesTotal)
			return nil
		default:
			logger.Println("POST read body error", filePath, readErr.Error())
			return readErr
		}
	}
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	filePath := getFullFilePath(r.URL.Path)
	if !fileExists(filePath) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err := os.Remove(filePath)
	if err != nil {
		logger.Println("DELETE error", filePath, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func optionsHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	filePath := getFullFilePath(r.URL.Path)
	if !fileExists(filePath) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	_, uploading := uploadMutex.Load(uri)
	if uploading {
		if err := serveUploadingFile(w, r); err != nil {
			msg := fmt.Sprintf("Serve uploading file error %s:%s", uri, err.Error())
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		return
	}

	http.ServeFile(w, r, filePath)
}

func serveUploadingFile(w http.ResponseWriter, r *http.Request) error {
	uri := r.URL.Path
	filePath := getFullFilePath(uri)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			logger.Println("Close file error", f.Name(), err.Error())
		}
	}(file)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("expected http.ResponseWriter to be an http.Flusher")
	}

	connectionClosed := false
	go func() {
		<-r.Context().Done()
		connectionClosed = true
	}()

	var readOffset int64
	for {
		if connectionClosed {
			logger.Println(fmt.Sprintf("GET %s conection closed %d", filePath, readOffset))
			return nil
		}

		fi, err := file.Stat()
		if err != nil {
			return err
		}

		readBufferSizeBytes := fi.Size() - readOffset + 1
		if readBufferSizeBytes > MaxReadBufferSizeBytes {
			readBufferSizeBytes = MaxReadBufferSizeBytes
		}

		buffer := make([]byte, readBufferSizeBytes)
		readBytes, readErr := file.ReadAt(buffer, readOffset)
		if readBytes != 0 {
			readOffset += int64(readBytes)
			if _, writeErr := w.Write(buffer[:readBytes]); writeErr != nil {
				logger.Println("HTTP write error ", writeErr)
				return writeErr
			}
			flusher.Flush()
		}

		switch readErr {
		case nil:
			continue
		case io.EOF:
			_, uploading := uploadMutex.Load(uri)
			if uploading {
				continue
			}
			logger.Println("GET EOF", filePath, readOffset)
			return nil
		default:
			msg := fmt.Sprintf("file.ReadAt error:%s", readErr.Error())
			logger.Println(msg)
			return readErr
		}
	}
}

func setCorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Headers", "Origin,Range,Accept-Encoding,Referer")
	w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func fileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getFullFilePath(f string) string {
	return DataDir + f
}
