package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)

var key = flag.String("key", "", "header Authentication:")

const (
	Authentication  = "Authentication"
	ContentType     = "Content-Type"
	ApplicationJson = "application/json"
)

func main() {
	host := flag.String("host", "0.0.0.0", "host to bind")
	port := flag.Int("port", 8000, "port to bind")

	addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("running d2 server at %s", addr)
	http.HandleFunc("/d2", httpPostHandler)
	http.HandleFunc("/d2/png", func(w http.ResponseWriter, r *http.Request) {
		handleQuery(w, r, func(data string) (string, error) {
			return data, nil
		}, false)
	})
	http.HandleFunc("/d2/png/base64", func(w http.ResponseWriter, r *http.Request) {
		handleQuery(w, r, func(data string) (string, error) {
			decoded, err := base64.RawURLEncoding.DecodeString(data)
			return string(decoded), err
		}, false)
	})
	http.HandleFunc("/d2/png/hex", func(w http.ResponseWriter, r *http.Request) {
		handleQuery(w, r, func(data string) (string, error) {
			decoded, err := hex.DecodeString(data)
			return string(decoded), err
		}, false)
	})
	http.ListenAndServe(addr, nil)
}

type JsonRequest struct {
	Diagram string `json:"diagram"`
}

type Response struct {
	Png   string `json:"png,ommitempty"`
	Svg   string `json:"svg,ommitempty"`
	Error string `json:"error,ommitempty"`
}

func handleQuery(w http.ResponseWriter, r *http.Request, decoder func(data string) (string, error), asJson bool) {
	if *key != "" {
		// check auth
		keyHeader := r.Header.Get(Authentication)
		if keyHeader != *key {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var d2DiagramData string
	d2DiagramData = r.URL.Query().Get("diagram")
	if len(d2DiagramData) == 0 {
		dropError(w, r, fmt.Errorf("empty diagram"), false)
		return
	}
	log.Printf("diagram param len: %d", len(d2DiagramData))
	d2DiagramData, err := decoder(d2DiagramData)
	if err != nil {
		dropError(w, r, err, false)
		return
	}
	log.Printf("diagram query param decoded len: %d", len(d2DiagramData))
	if len(d2DiagramData) == 0 {
		dropError(w, r, err, false)
		return
	}
	processRequest(w, r, d2DiagramData, asJson)
}

func httpPostHandler(w http.ResponseWriter, r *http.Request) {
	if *key != "" {
		// check auth
		keyHeader := r.Header.Get(Authentication)
		if keyHeader != *key {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// read request
	defer r.Body.Close()
	requestData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var d2DiagramData string
	if r.Header.Get(ContentType) == ApplicationJson {
		log.Printf(ApplicationJson)
		var req JsonRequest
		err = json.Unmarshal(requestData, &req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		d2DiagramData = req.Diagram
	} else {
		log.Printf("raw POST")
		d2DiagramData = string(requestData)
	}
	processRequest(w, r, d2DiagramData, true)
}

func dropError(w http.ResponseWriter, r *http.Request, err error, asJson bool) {
	var resp Response
	log.Printf("error: %v", err)
	if asJson {
		encoder := json.NewEncoder(w)
		resp.Error = err.Error()
		encoder.Encode(&resp)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}

func processRequest(w http.ResponseWriter, r *http.Request, d2DiagramData string, asJson bool) {
	tmp, err := os.CreateTemp("/tmp", "*.png")
	if err != nil {
		dropError(w, r, err, asJson)
		return
	}
	defer tmp.Close()
	cmd := exec.Command("d2", "-", tmp.Name())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		dropError(w, r, err, asJson)
		return
	}
	err = cmd.Start()
	if err != nil {
		dropError(w, r, err, asJson)
		return
	}
	errChan := make(chan error)
	defer close(errChan)
	go func() {
		log.Printf("sending '%s' to d2", d2DiagramData)
		_, err := stdin.Write([]byte(d2DiagramData))
		log.Printf("diagram was sent")
		errChan <- err
	}()
	select {
	case err = <-errChan:
		{
			if err != nil {
				dropError(w, r, err, asJson)
				return

			}
		}
	}
	log.Printf("waiting for ending d2")
	stdin.Close()
	err = cmd.Wait()
	if err != nil {
		dropError(w, r, err, asJson)
		return
	}
	log.Printf("reading result")
	png, err := ioutil.ReadFile(tmp.Name())
	if err != nil {
		dropError(w, r, err, asJson)
		return
	}

	log.Printf("rendered ? %v", err)
	if err != nil {
		dropError(w, r, err, asJson)
		return
	}
	log.Printf("bytes %v", len(png))
	w.WriteHeader(http.StatusOK)
	if asJson {
		resp := Response{}
		resp.Svg = string(png)
		encoder := json.NewEncoder(w)
		_ = encoder.Encode(&resp)
	} else {
		w.Write(png)
	}
	return
}
