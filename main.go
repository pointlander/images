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
	"image"
	_ "image/gif"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nfnt/resize"
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
	<table id="thumbs">
	</table>
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
	 function change(id) {
     img.src = "images/" + images[id];
		 console.log(img.src);
   }
	 function handler(id) {
		 return function (){change(id)};
	 }
	 var tr = null;
	 for (var i = 0; i < images.length; i++) {
		 if ((i % 5) == 0) {
			if (tr != null) {
				var thumbs = document.getElementById("thumbs")
				thumbs.appendChild(tr);
			}
		  tr = document.createElement("tr");
		 }
		 var img1 = document.createElement("img");
		 var image = ` + "`" + `${images[i]}` + "`" + `;
		 var parts = image.split('.');
		 img1.src = "thumbs/" + parts[0] + ".jpeg";
		 images[i] = parts.join('.');
		 img1.onclick = handler(i);
		 var td = document.createElement("td");
		 td.appendChild(img1);
		 tr.appendChild(td);
	 }
	 if ((images.length % 5) != 0) {
		 var thumbs = document.getElementById("thumbs")
		 thumbs.appendChild(tr);
	 }
	 for (var i = 0; i < images.length; i++) {
		 console.log(images[i]);
	 }
  </script>
 </body>
</html>
`
	// Address is the address of the server
	Address = flag.String("address", ":80", "address of the server")
	// Fetch fetch url
	Fetch = flag.String("fetch", "", "file to fetch")
	// Thumb generate thumb nails
	Thumb = flag.Bool("thumb", false, "generate thumb nails")
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

	if *Thumb {
		dir, err := os.Open("imgs/")
		if err != nil {
			panic(err)
		}
		names, err := dir.Readdirnames(-1)
		if err != nil {
			panic(err)
		}
		sort.Strings(names)
		_, err = os.Stat("thumbs")
		if err != nil {
			err = os.Mkdir("thumbs", 0755)
			if err != nil {
				panic(err)
			}
		}
		for _, name := range names {
			thumb := strings.TrimSuffix(name, ".gif") + ".jpeg"
			_, err := os.Stat("thumbs/" + thumb)
			if err == nil {
				continue
			}
			input, err := os.Open("imgs/" + name)
			if err != nil {
				panic(err)
			}
			img, _, err := image.Decode(input)
			if err != nil {
				panic(err)
			}
			input.Close()
			resized := resize.Resize(128, 0, img, resize.Lanczos3)

			output, err := os.Create("thumbs/" + thumb)
			if err != nil {
				panic(err)
			}
			err = jpeg.Encode(output, resized, nil)
			if err != nil {
				panic(err)
			}
			output.Close()
		}
		return
	}

	router := httprouter.New()
	router.GET("/", handleError(routeIndex))
	router.GET("/images/:image", handleError(routeImage))
	router.GET("/thumbs/:image", handleError(routeThumbs))

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

func routeThumbs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) error {
	file := filepath.Base(ps[0].Value)
	if file == ".." {
		return errors.New("file not found")
	}

	w.Header().Set("Content-Type", "image/jpeg")
	data, err := ioutil.ReadFile("thumbs/" + file)
	if err != nil {
		return err
	}
	w.Write(data)

	return nil
}
