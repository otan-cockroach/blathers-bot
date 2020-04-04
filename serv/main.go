package main

import (
	"log"
	"net/http"

	"github.com/otan-cockroach/blathers-bot"
)

// TODO(otan): consider using gin instead.
func main() {
	http.HandleFunc("/github_webhook", blathers.Server().HandleGithubWebhook)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
