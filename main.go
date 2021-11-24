// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	icingaTimet := flag.Int64("icinga.timet", 0, "$icinga.timet$")

	hName := flag.String("host.name", "", "$host.name$")
	hDisplayName := flag.String("host.display_name", "", "$host.display_name$")
	hActionUrl := flag.String("host.action_url", "", "$host.action_url$")
	hState := flag.String("host.state", "", "$host.state$")
	hOutput := flag.String("host.output", "", "$host.output$")

	sName := flag.String("service.name", "", "$service.name$")
	sDisplayName := flag.String("service.display_name", "", "$service.display_name$")
	sActionUrl := flag.String("service.action_url", "", "$service.action_url$")
	sState := flag.String("service.state", "", "$service.state$")
	sOutput := flag.String("service.output", "", "$service.output$")

	flag.Parse()

	if *icingaTimet == 0 {
		*icingaTimet = time.Now().Unix()
	}

	webhook := os.Getenv("ROCKETCHAT_WEBHOOK_URL")
	if empty(webhook) {
		complain("$ROCKETCHAT_WEBHOOK_URL missing")
	}

	reportService := true
	state := *sState
	output := *sOutput

	if !empty(*sName) || !empty(*sDisplayName) || !empty(*sActionUrl) || !empty(*sState) || !empty(*sOutput) {
		if empty(*hName) || empty(*sName) || empty(*sState) {
			complain("-service.* is given, missing some of: -host.name, -service.name, -service.state")
		}

		if empty(*sDisplayName) {
			sDisplayName = sName
		}
	} else if !empty(*hName) || !empty(*hDisplayName) || !empty(*hActionUrl) || !empty(*hState) || !empty(*hOutput) {
		if empty(*hName) || empty(*hState) {
			complain("-host.* is given, missing some of: -host.name, -host.state")
		}

		reportService = false
		state = *hState
		output = *hOutput
	} else {
		complain("Missing either -host.name and -host.state or -host.name, -service.name and -service.state")
	}

	if empty(*hDisplayName) {
		hDisplayName = hName
	}

	webhookUrl, errUP := url.Parse(webhook)
	if errUP != nil {
		complain(errUP.Error())
	}

	exitSuccess := 0

	hostname, errHn := os.Hostname()
	if errHn != nil {
		_, _ = fmt.Fprintln(os.Stderr, errHn.Error())
		exitSuccess = 1

		hostname = "(unknown)"
	}

	icon := "question"
	pMark := "!"

	switch state {
	case "UP", "OK":
		icon = "white_check_mark"
		pMark = "."
	case "WARNING":
		icon = "warning"
	case "DOWN", "CRITICAL":
		icon = "exclamation"
	}

	buf := &strings.Builder{}
	if reportService {
		mustFprintf(buf, ":%s: *Service monitoring on %s* :%s:", icon, hostname, icon)

		mustFprintf(
			buf, "\n\n%s on %s is *%s*%s",
			linkOrItalic(*sDisplayName, *sActionUrl),
			linkOrItalic(*hDisplayName, *hActionUrl), strings.ToLower(state), pMark,
		)

		mustFprintf(buf, "\n\nWhen: %s\nHost: %s\nService: %s", time.Unix(*icingaTimet, 0), *hName, *sName)
	} else {
		mustFprintf(buf, ":%s: *Host monitoring on %s* :%s:", icon, hostname, icon)
		mustFprintf(buf, "\n\n%s is *%s*%s", linkOrItalic(*hDisplayName, *hActionUrl), strings.ToLower(state), pMark)
		mustFprintf(buf, "\n\nWhen: %s\nHost: %s", time.Unix(*icingaTimet, 0), *hName)
	}

	mustFprintf(buf, "\nInfo:\n\n```\n%s\n```", output)

	body := &bytes.Buffer{}
	je := json.NewEncoder(body)

	je.SetEscapeHTML(false)

	if err := je.Encode(map[string]interface{}{"text": buf.String()}); err != nil {
		panic(err)
	}

	response, errRq := http.DefaultClient.Do(&http.Request{
		Method: "POST",
		URL:    webhookUrl,
		Header: map[string][]string{"Content-Type": {"application/json"}},
		Body:   io.NopCloser(body),
	})
	if errRq != nil {
		_, _ = fmt.Fprintln(os.Stderr, errRq.Error())
		os.Exit(1)
	}

	if response.StatusCode > 299 {
		_ = response.Write(os.Stderr)
		os.Exit(1)
	}

	os.Exit(exitSuccess)
}

func empty(s string) bool {
	return strings.TrimSpace(s) == ""
}

func complain(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}

func mustFprintf(w io.Writer, format string, a ...interface{}) {
	if _, err := fmt.Fprintf(w, format, a...); err != nil {
		panic(err)
	}
}

func linkOrItalic(text, url string) string {
	if empty(url) {
		return fmt.Sprintf("_%s_", text)
	}

	return fmt.Sprintf("[%s](%s)", text, url)
}
