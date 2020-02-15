package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/progrium/hotweb/pkg/hotweb"
	"github.com/skratchdot/open-golang/open"
)

var (
	Port string
	Dir  string
)

func init() {
	flag.StringVar(&Port, "port", "8080", "port to listen on")
	flag.StringVar(&Dir, "dir", ".", "directory to serve")
}

func main() {
	flag.Parse()

	var err error
	if Dir == "." {
		Dir, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	}

	hw := hotweb.New(Dir, nil)

	go func() {
		log.Printf("watching %#v\n", Dir)
		log.Fatal(hw.Watch())
	}()

	listenAddr := "0.0.0.0:" + Port
	url := "http://" + listenAddr
	open.Start(url)

	log.Printf("serving at %s\n", url)
	http.ListenAndServe(listenAddr, handlers.LoggingHandler(os.Stdout, hw))
}
