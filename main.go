package main

// Utility service to evaluate OPA policies against a Terraform plan and update a GH PR
// Intended to run as a sidecar to Atlantis (see https://www.runatlantis.io)
//
// - Run as a HTTP server on a given port
// - Endpoint to accept a TF plan in JSON format, a query and a GH Pull Request URL
// - Execute the query
// - Update pull request status based on query result (pass/fail)
//
// Inspired by https://marcyoung.us/post/atlantis-opa/, except that this is a sidecar and not
// a Lambda function.

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/open-policy-agent/opa/rego"
)

const query = "pass = data.terraform.tagging.pass; violations = data.terraform.tagging.violations"
const policy = "./policies"
const addr = "localhost:8080"

const prRepo = "opa-terraform-example"
const prNo = 1
const repoOwner = "joerx"

type errorResponse struct {
	Error string `json:"error"`
}

type regoHandler struct {
	rego *rego.Rego
	gh   *github.Client
}

func main() {
	githubToken, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		log.Fatal("Missing GITHUB_TOKEN")
	}

	rg := rego.New(
		rego.Query(query),
		rego.Load([]string{policy}, nil),
	)

	gh := newGithubClient(githubToken)
	rh := &regoHandler{rego: rg, gh: gh}

	router := http.NewServeMux()
	srv := http.Server{
		Addr:    addr,
		Handler: router,
	}

	router.HandleFunc("/", rh.handleRequest)

	log.Printf("Starting server on %s", addr)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen %s: %s", addr, err)
	}
}

func (rh *regoHandler) handleRequest(w http.ResponseWriter, req *http.Request) {
	log.Printf("%s %s - %s", req.Method, req.RemoteAddr, req.URL.Path)

	input, err := parseInput(req.Body)
	if err != nil {
		respondInternalServerError(w, err)
		return
	}

	rs, err := rh.checkForViolations(input)
	if err != nil {
		respondInternalServerError(w, err)
		return
	}

	respondOK(w, rs)
}

func (rh *regoHandler) checkForViolations(input interface{}) (interface{}, error) {
	rs, err := rh.evalQuery(policy, query, input)
	if err != nil {
		return nil, err
	}

	if err := rh.updateGithubStatus(rs); err != nil {
		return nil, err
	}

	return rs, err
}

func (rh *regoHandler) evalQuery(policy, q string, input interface{}) (rego.ResultSet, error) {
	ctx := context.Background()

	query, err := rh.rego.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	return query.Eval(ctx, rego.EvalInput(input))
}

// Github functions

func makeIssueComment(violations map[string]interface{}) *github.IssueComment {
	lines := []string{
		"Some policy checks failed, please fix them and then re-run `atlantis plan`:",
		"",
		"```txt",
	}

	for addr, elem := range violations {
		msg := elem.(string)
		lines = append(lines, fmt.Sprintf("%s: %s", addr, msg))
	}

	lines = append(lines, "```")

	return &github.IssueComment{
		Body: github.String(strings.Join(lines, "\n")),
	}
}

func makeRepoStatus(pass bool, violations map[string]interface{}) *github.RepoStatus {
	state := "failure"
	description := "Policy checks failed, please see issue comment for detail"
	context := "opa/policy-check"

	if pass {
		state = "success"
		description = "Policy checks passed :)"
	}

	return &github.RepoStatus{
		State:       github.String(state),
		Description: github.String(description),
		Context:     github.String(context),
	}
}

func (rh *regoHandler) updateGithubStatus(rs rego.ResultSet) error {
	pass := rs[0].Bindings["pass"].(bool)
	violations := rs[0].Bindings["violations"].(map[string]interface{})

	log.Printf("Pass: %t (%d violations)\n", pass, len(violations))

	ctx := context.Background()

	pr, _, err := rh.gh.PullRequests.Get(ctx, repoOwner, prRepo, prNo)
	if err != nil {
		return fmt.Errorf("error getting PR info: %v", err)
	}

	ref := pr.GetHead().SHA

	if !pass {
		comment := makeIssueComment(violations)
		_, _, err := rh.gh.Issues.CreateComment(ctx, repoOwner, prRepo, prNo, comment)
		if err != nil {
			log.Println(err)
			return fmt.Errorf("error adding issue comment")
		}
	}

	status := makeRepoStatus(pass, violations)
	_, _, err = rh.gh.Repositories.CreateStatus(ctx, repoOwner, prRepo, *ref, status)
	if err != nil {
		log.Println(err)
		return fmt.Errorf("error updating PR status")
	}

	return nil
}
