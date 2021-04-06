// Copyright 2021 The Images Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"time"

	"github.com/julienschmidt/httprouter"
)

// ErrorHandle handler that can return error
type ErrorHandle func(http.ResponseWriter, *http.Request, httprouter.Params) error

// Error is an error
type Error struct {
	Error string `json:"error"`
}

// IndexTemplateValues is the values for the index template
type IndexTemplateValues struct {
	Images []string
}

var (
	// IndexTemplate is the template of the index page
	IndexTemplate = `
<html>
 <head>
  <title>Images</title>
 </head>
 <body>
  <img id="img"/><br/><br>
  <button onclick="previous()">Previous</button> <button onclick="next()">Next</button>
  <script>
   var images = [
  {{range .Images}}
    "{{.}}",
  {{end}}
   ];
   var current = 0;
   var img = document.getElementById("img");
   img.src = "images/" + images[current];
   function next() {
     current = (current + 1) % images.length;
     img.src = "images/" + images[current];
   }
   function previous() {
     current = (images.length + current - 1) % images.length;
     img.src = "images/" + images[current];
   }
  </script>
 </body>
</html>
`
	// Address is the address of the server
	Address = flag.String("address", ":80", "address of the server")
	// Fetch fetch url
	Fetch = flag.String("fetch", "", "file to fetch")
	// IndexTemplateInstance is an instance of an index templates
	IndexTemplateInstance *template.Template
)

func init() {
	var err error
	IndexTemplateInstance, err = template.New("index").Parse(IndexTemplate)
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()

	if *Fetch != "" {
		response, err := http.Get(*Fetch)
		if err != nil {
			panic(err)
		}
		data, err := ioutil.ReadAll(response.Body)
		if err != nil {
			panic(err)
		}
		response.Body.Close()
		fmt.Println(string(data))
		return
	}

	router := httprouter.New()
	router.GET("/", handleError(routeIndex))
	router.GET("/images/:image", handleError(routeImage))

	server := http.Server{
		Addr:           *Address,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	go func() {
		for range signals {
			server.Shutdown(context.Background())
		}
	}()
	server.ListenAndServe()
}

func handleError(handler ErrorHandle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		err := handler(w, r, ps)
		if err != nil {
			response, erri := json.Marshal(Error{Error: err.Error()})
			if erri != nil {
				panic(erri)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(response)
		}
	}
}

func routeIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	w.Header().Set("Content-Type", "text/html")

	dir, err := os.Open("imgs/")
	if err != nil {
		return err
	}
	directories, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	sort.Strings(directories)
	index := IndexTemplateValues{
		Images: directories,
	}
	err = IndexTemplateInstance.Execute(w, index)
	if err != nil {
		return err
	}

	return nil
}

func routeImage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	file := filepath.Base(ps[0].Value)
	if file == ".." {
		return errors.New("file not found")
	}

	w.Header().Set("Content-Type", "image/gif")
	data, err := ioutil.ReadFile("imgs/" + file)
	if err != nil {
		return err
	}
	w.Write(data)

	return nil
}
