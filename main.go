package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

const (
	defaultTemplate = `<!DOCTYPE html>
<html>

<head>
  <meta http-equiv="content-type" content="text/html; charset=utf-8">
  <title>{{.Title}}:{{.Filename}}</title>
</head>

<body>
{{.Body}}
</body>

</html>
`
)

type content struct {
	Filename string
	Title    string
	Body     template.HTML
}

func main() {
	filename := flag.String("file", "", "Markdown file to preview")
	skipPreview := flag.Bool("s", false, "Skip auto-preview")
	tFname := flag.String("t", "", "Alternate template name")
	pipe := flag.Bool("pipe", false, "for use stdin")
	flag.Parse()

	dt := os.Getenv("MDP_TEMPLATE")
	if dt != "" {
		*tFname = dt
	}

	if *filename == "" && !*pipe {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(*filename, *tFname, os.Stdout, *skipPreview, *pipe); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(filename, tFname string, out io.Writer, skipPreview, pipe bool) error {
	var input io.Reader
	var err error
	if pipe {
		input = os.Stdin
	} else {
		input, err = os.Open(filename)
		if err != nil {
			return err
		}

	}

	htmlData, err := parseContent(input, tFname, filename)
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp("", "mdp*.html")
	if err != nil {
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	outName := temp.Name()

	fmt.Fprintln(out, outName)

	if err := saveHTML(outName, htmlData); err != nil {
		return err
	}

	if skipPreview {
		return nil
	}

	defer os.Remove(outName)

	return preview(outName)
}

func parseContent(r io.Reader, tFname string, filename string) ([]byte, error) {
	input, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	output := blackfriday.Run(input)
	body := bluemonday.UGCPolicy().SanitizeBytes(output)

	t, err := template.New("mdp").Parse(defaultTemplate)
	if err != nil {
		return nil, err
	}

	if tFname != "" {
		t, err = template.ParseFiles(tFname)
		if err != nil {
			return nil, err
		}
	}

	c := content{
		Filename: filepath.Base(filename),
		Title:    "Markdown Preview Tool",
		Body:     template.HTML(body),
	}

	var buffer bytes.Buffer

	if err := t.Execute(&buffer, c); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

const (
	// 0644 is readable and writeable by the owner but only readable by anyone else.
	FilePermissionReadWrite = 0644
)

func saveHTML(outFname string, data []byte) error {
	return os.WriteFile(outFname, data, FilePermissionReadWrite)
}

// fname = filename, cName command name
func preview(fname string) error {
	cName := ""
	cParams := []string{}

	switch runtime.GOOS {
	case "linux":
		cName = "xdg-open"
	case "windows":
		cName = "cmd.exe"
		cParams = []string{"/C", "start"}
	case "darwin":
		cName = "open"
	default:
		return fmt.Errorf("OS not supported")
	}

	cParams = append(cParams, fname)
	cPath, err := exec.LookPath(cName)
	if err != nil {
		return err
	}

	err = exec.Command(cPath, cParams...).Run()

	// give the browser some time to open  the file before deleting it
	time.Sleep(2 * time.Second)
	return err
}
