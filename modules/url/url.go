// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public
// License along with this program. If not, see <http://www.gnu.org/licenses/>.

package url

import (
	"errors"
	"fmt"
	"github.com/nmeum/marvin/irc"
	"github.com/nmeum/marvin/modules"
	"html"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var (
	headerError  = errors.New("missing content-type header")
	extractError = errors.New("couldn't extract title")
)

type Module struct {
	re      *regexp.Regexp
	Regex   string   `json:"regex"`
	Exclude []string `json:"exclude"`
}

func Init(moduleSet *modules.ModuleSet) {
	moduleSet.Register(new(Module))
}

func (m *Module) Name() string {
	return "url"
}

func (m *Module) Help() string {
	return "Displays HTML titles for HTTP links."
}

func (m *Module) Defaults() {
	m.Regex = `(http|https)\://[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,3}(:[a-zA-Z0-9]*)?/?([a-zA-Z0-9\-\._\?\,\'/\\\+&amp;%\$#\=~])*`
}

func (m *Module) Load(client *irc.Client) error {
	re, err := regexp.Compile(m.Regex)
	if err != nil {
		return err
	}

	m.re = re
	client.CmdHook("privmsg", m.urlCmd)

	return nil
}

func (m *Module) urlCmd(client *irc.Client, msg irc.Message) error {
	link := m.re.FindString(msg.Data)
	if len(link) <= 0 {
		return nil
	}

	purl, err := url.Parse(link)
	if err != nil || m.isExcluded(purl.Host) {
		return nil
	}

	resp, err := http.Get(purl.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	info, err := m.infoString(resp)
	if err != nil {
		return err
	}

	return client.Write("NOTICE %s :%s", msg.Receiver, info)
}

func (m *Module) infoString(resp *http.Response) (info string, err error) {
	ctype := resp.Header.Get("Content-Type")
	if len(ctype) <= 0 {
		err = headerError
		return
	}

	mtype, _, err := mime.ParseMediaType(ctype)
	if err != nil {
		return
	}

	info = fmt.Sprintf("URL -- Type: %s", mtype)
	csize := resp.Header.Get("Content-Length")
	if len(csize) > 0 {
		info = fmt.Sprintf("%s. Size: %s bytes", info, csize)
	}

	if mtype == "text/html" {
		title, err := m.extractTitle(resp.Body)
		if err == nil {
			info = fmt.Sprintf("%s. Title: %s", info, title)
		}
	}

	return
}

func (m *Module) extractTitle(body io.ReadCloser) (title string, err error) {
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return
	}

	regex := regexp.MustCompile("(?is)<title>(.+)</title>")
	match := regex.Find(data)
	if len(match) <= 0 {
		return "", extractError
	}

	title = string(match)
	title = title[len("<title>"):strings.Index(title, "</title>")]

	title = m.sanitize(html.UnescapeString(title))
	if len(title) <= 0 {
		return "", extractError
	}

	return
}

func (m *Module) sanitize(title string) string {
	normalized := strings.Replace(title, "\n", " ", -1)
	for strings.Contains(normalized, "  ") {
		normalized = strings.Replace(normalized, "  ", " ", -1)
	}

	return strings.TrimSpace(normalized)
}

func (m *Module) isExcluded(host string) bool {
	for _, h := range m.Exclude {
		if host == h {
			return true
		}
	}

	return false
}
