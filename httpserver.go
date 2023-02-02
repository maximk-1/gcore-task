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
	logger      *log.Logger
	workingDir  string
	uploadMutex sync.Map
)

const RW_BUFFER_SIZE = 1024 * 1024

func main() {
	logger = log.Default()
	var err error
	workingDir, err = os.Getwd()
	if err != nil {
		logger.Panic(err)
	}

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

    logger.Println("Server start");
	logger.Fatal(http.ListenAndServe(":80", nil))
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	filePath := getFullFilePath(r.URL.Path)

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logger.Println("POST openfile err", filePath, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logger.Println("POST closefile err", filePath, err.Error())
		}
	}(file)

	uploadMutex.Store(filePath, true)
	defer func() {
		uploadMutex.Delete(filePath)
	}()

	var totalReadBytes int
	for {
		buffer := make([]byte, 2*RW_BUFFER_SIZE)
		readBytes, err := r.Body.Read(buffer)

		if readBytes != 0 {
			totalReadBytes += readBytes
			_, err = file.Write(buffer[:readBytes])
			if err != nil {
				logger.Println("Write file error", filePath)
				return
			}
		}

		switch err {
		case nil:
			continue
		case io.EOF:
			logger.Println("POST EOF", filePath, totalReadBytes)
			w.WriteHeader(http.StatusOK)
			return
		default:
			logger.Println("POST err", filePath, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
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
	filePath := getFullFilePath(r.URL.Path)
	if !fileExists(filePath) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	_, uploading := uploadMutex.Load(filePath)
	if fileExists(filePath) && !uploading {
		http.ServeFile(w, r, filePath)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		msg := fmt.Sprintf("Error reading file %s:%s", filePath, err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			logger.Println("Closefile error", f.Name(), err.Error())
		}
	}(file)

	flusher, ok := w.(http.Flusher)
	if !ok {
		msg := fmt.Sprintf("expected http.ResponseWriter to be an http.Flusher")
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	connectionClosed := false
	go func() {
		<-r.Context().Done()
		connectionClosed = true
	}()

	var offset int64
	for {
		if connectionClosed {
			logger.Println(fmt.Sprintf("%s conection closed", filePath))
			return
		}

		buffer := make([]byte, RW_BUFFER_SIZE)
		readBytes, readErr := file.ReadAt(buffer, offset)
		if readBytes != 0 {
			offset += int64(readBytes)
			if _, writeErr := w.Write(buffer[:readBytes]); writeErr != nil {
				logger.Println("Write error ", writeErr)
				return
			}
			flusher.Flush()
		}

		switch readErr {
		case nil:
			continue
		case io.EOF:
			_, uploading := uploadMutex.Load(filePath)
			if uploading {
				continue
			}
			logger.Println(fmt.Sprintf("%s read done %d", filePath, offset+int64(readBytes)))
			return
		default:
			msg := fmt.Sprintf("file.ReadAt error:%s", readErr.Error())
			logger.Println(msg)
			return
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
	return workingDir + f
}
