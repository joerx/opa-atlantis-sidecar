package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func parseInput(r io.ReadCloser, out interface{}) error {
	defer r.Close()

	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	// var input interface{}
	if err := json.Unmarshal(bs, &out); err != nil {
		return err
	}

	return nil
}

func respond(w http.ResponseWriter, code int, payload interface{}) {
	bytes, _ := json.Marshal(payload)
	w.WriteHeader(code)
	w.Header().Add("Content-type", "application/json")
	w.Write(bytes)
}

func respondOK(w http.ResponseWriter, payload interface{}) {
	respond(w, http.StatusOK, payload)
}

func respondAccepted(w http.ResponseWriter, payload interface{}) {
	respond(w, http.StatusAccepted, payload)
}

func respondInternalServerError(w http.ResponseWriter, err error) {
	respond(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("%s", err)})
}

func handleError(err error) {
	log.Fatal(err)
}
