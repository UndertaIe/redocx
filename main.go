package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/exp/maps"
)

var (
	generateStr  = "npx @redocly/cli build-docs -o %s/%s %s"
	baseDir      = "api_docs"
	copyStr      = "http://%s:28888/%s"
	netInterface = "en0"
	addr         = ":28888"
)

// eg. go run main.go admin_api:../admin-api/api/http/openapi.yaml open_api:../open-api/third_party/swagger_ui/dist/openapi.yaml
// go run main.go 输出目录:输入目录 输出目录2:输入目录2
// go run main.go admin:admin-api/openapi.yaml admin2:admin-api/openapi2.yaml
func main() {
	log.Println("redocx is running!")
	watchFiles := parseArgs()

	go serve(baseDir)
	go watch(watchFiles)

	<-make(chan struct{})
}

func parseArgs() []string {
	args := os.Args[1:]
	for _, s := range args {
		ss := strings.Split(s, ":")
		if len(ss) != 2 {
			log.Panicln("parse args error")
		}
		output, input := ss[0], ss[1]
		docMap[input] = output

		if _, err := os.Stat(input); os.IsNotExist(err) {
			fmt.Printf("fn: %v\n", input)
			log.Fatalln(input + "is not exist")
		}
	}
	return maps.Keys(docMap)
}

func watch(watchFiles []string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	for _, fn := range watchFiles {
		if err := watcher.Add(fn); err != nil {
			log.Fatalln(err)
		}
		log.Println("WATCH: ", fn)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if ok {
				if event.Op.Has(fsnotify.Write) || event.Op.Has(fsnotify.Create) {
					log.Printf("generate %s doc is running!\n", event.Name)
					now := time.Now()
					output := docPath(event.Name)
					if updateDoc(event.Name) != nil {
						log.Panicln("generate doc error:", err)
					}
					link := shareLink(output)
					log.Println("docs is generated, share link:) ", link)
					log.Printf("elapsed time: %fs\n", time.Since(now).Seconds())
					copyToClipBoard(link)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

var docMap = make(map[string]string)

func serve(servePath string) {
	if _, err := os.Stat(servePath); os.IsNotExist(err) {
		if err := os.Mkdir(servePath, os.ModePerm); err != nil {
			log.Panicln(err)
		}
	}

	if err := http.ListenAndServe(addr, http.FileServer(http.Dir(servePath))); err != nil {
		log.Panicln(err)
	}
}

func updateDoc(fn string) error {
	args := strings.Split(fmt.Sprintf(generateStr, baseDir, docPath(fn), fn), " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func docPath(path string) string {
	_, name := filepath.Split(path)
	name = strings.TrimSuffix(name, ".yaml")

	dir := docMap[path]
	if dir != "" {
		return dir + "/" + name + ".html"
	}
	return name
}

func copyToClipBoard(s string) {
	if err := clipboard.WriteAll(s); err != nil {
		log.Println("copy to clipboard error:(", err.Error())
	}
}

func shareLink(s string) string {
	host, err := getIPAddress(netInterface)
	if err != nil {
		log.Fatalln(err)
	}
	return fmt.Sprintf(copyStr, host, strings.ReplaceAll(s, ".yaml", ".html"))
}

func getIPAddress(interfaceName string) (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Name == interfaceName {
			addrs, err := iface.Addrs()
			if err != nil {
				return "", err
			}

			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
					return ipNet.IP.String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("ip address not found")
}
