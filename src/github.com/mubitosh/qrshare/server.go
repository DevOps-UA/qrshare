package main

import (
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type srvFlag struct {
	value bool
	mutex sync.Mutex
}

func (f *srvFlag) set(v bool) {
	f.mutex.Lock()
	f.value = v
	f.mutex.Unlock()
}

func (f *srvFlag) get() bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.value
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

// fileServer serves a file on a random port number. It shuts down if there
// is no download from the server within a period of App.inactiviy seconds.
type fileServer struct {
	http.Server
	port     int
	listener net.Listener
}

func fileServerNew() (*fileServer, error) {
	fs := &fileServer{}
	fs.Server.Addr = ":"
	listener, err := net.Listen("tcp", fs.Server.Addr)
	fs.listener = listener
	if err != nil {
		return nil, err
	}
	fs.port = fs.listener.Addr().(*net.TCPAddr).Port
	return fs, nil
}

func (fs *fileServer) start(app *QrShare, qrWindow *gtk.ApplicationWindow) error {
	serving, justServed := new(srvFlag), new(srvFlag)
	serving.set(false)
	justServed.set(false)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serving.set(true)
		log.Println("Serving file:", *app.file)
		http.ServeFile(w, r, *app.file)
		log.Println("File served")
		serving.set(false)
		justServed.set(true)
	})

	fs.Server.Handler = mux

	go func() {
		for {
			justServed.set(false)
			time.Sleep(time.Duration(*app.inActive) * time.Second)
			if !serving.get() && !justServed.get() {
				log.Println("Exceeded inactive time of", *app.inActive, "seconds")
				log.Println("Stopping file sharing")
				if app.isContractor {
					log.Println("App was started to display QR window only, exiting app")
					os.Exit(0)
				}
				log.Println("App was started with main window, back to main window")
				glib.IdleAdd(qrWindow.Destroy)
				return
			}
		}
	}()

	log.Println("Starting file sharing")
	return fs.Serve(tcpKeepAliveListener{fs.listener.(*net.TCPListener)})
}
