// XML, CSV, YAML and form-encoded bodies for the same Task.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"log"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

type Task struct {
	XMLName xml.Name `xml:"task" yaml:"-"`
	ID      int      `xml:"id,attr" yaml:"id"`
	Title   string   `xml:"title" yaml:"title"`
	Status  string   `xml:"status" yaml:"status"`
	Tags    []string `xml:"tags>tag" yaml:"tags,omitempty"`
}

func main() {
	t := Task{ID: 1, Title: "Write docs & ship", Status: "todo", Tags: []string{"docs", "p1"}}

	x, err := xml.MarshalIndent(t, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(xml.Header + string(x))
	var fromXML Task
	if err := xml.Unmarshal(x, &fromXML); err != nil {
		log.Fatal(err)
	}
	fmt.Println(fromXML.Title, fromXML.Tags)

	// CSV: the writer buffers, so Flush and check Error. It quotes fields only when needed.
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "title", "tags"})
	_ = w.Write([]string{"1", "Write docs, ship", strings.Join(t.Tags, ";")})
	w.Flush()
	if err := w.Error(); err != nil {
		log.Fatal(err)
	}
	fmt.Print(buf.String())
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(rows), rows[1][1])

	y, err := yaml.Marshal(t)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(y))
	var fromYAML Task
	if err := yaml.Unmarshal([]byte("id: 2\ntitle: From YAML\nstatus: done\n"), &fromYAML); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d %s %s %v\n", fromYAML.ID, fromYAML.Title, fromYAML.Status, fromYAML.Tags == nil)

	// application/x-www-form-urlencoded
	form := url.Values{}
	form.Set("title", "Buy milk & eggs")
	form.Add("tag", "home")
	form.Add("tag", "errands")
	encoded := form.Encode()
	fmt.Println(encoded)
	parsed, err := url.ParseQuery(encoded)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(parsed.Get("title"), parsed["tag"])
}
