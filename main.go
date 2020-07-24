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

const query = "pass = data.terraform.pass; violations = data.terraform.violations"
const prRepo = "opa-terraform-example"
const prNo = 1
const repoOwner = "joerx"

type errorResponse struct {
	Error string `json:"error"`
}

type messageResponse struct {
	Message string `json:"message"`
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

	var address string
	address, ok = os.LookupEnv("ADDRESS")
	if !ok {
		address = "localhost:5151"
	}

	var policyDir string
	policyDir, ok = os.LookupEnv("POLICY_DIR")
	if !ok {
		policyDir = "./policies"
	}

	log.Printf("Reading policies from %s", policyDir)
	rg := rego.New(
		rego.Query(query),
		rego.Load([]string{policyDir}, nil),
	)

	gh := newGithubClient(githubToken)
	rh := &regoHandler{rego: rg, gh: gh}

	router := http.NewServeMux()
	srv := http.Server{
		Addr:    address,
		Handler: router,
	}

	router.HandleFunc("/", rh.handleRequest)

	log.Printf("Starting server on %s", address)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen %s: %s", address, err)
	}
}

type prInfo struct {
	Name    string `json:"name"`
	Owner   string `json:"owner"`
	PullNum int    `json:"pull_num"`
}

type tfPlanRequest struct {
	PullRequest *prInfo `json:"pull_request"`
	Plan        interface{}
}

func (rh *regoHandler) handleRequest(w http.ResponseWriter, req *http.Request) {
	log.Printf("%s %s - %s", req.Method, req.RemoteAddr, req.URL.Path)

	var in tfPlanRequest
	if err := parseInput(req.Body, &in); err != nil {
		log.Printf("error parsing request body: %v", err)
		respondInternalServerError(w, err)
		return
	}

	respondAccepted(w, &messageResponse{Message: "accepted"})

	// we don't return the results, instead we report them back to the VCS provider
	err := rh.checkForViolations(&in)
	if err != nil {
		log.Println(err)
	}
}

func (rh *regoHandler) checkForViolations(input *tfPlanRequest) error {
	log.Println("Checking for violations")

	rs, err := rh.evalQuery(query, input.Plan)
	if err != nil {
		return err
	}

	log.Println("Check completed, result below")
	log.Printf("%#v", rs)

	if rs == nil {
		return fmt.Errorf("OPA returned an empty result") // TODO: should raise an error on Github
	}

	if input.PullRequest != nil {
		if err := rh.updateGithubStatus(rs, input.PullRequest); err != nil {
			return err
		}
	}

	return err
}

func (rh *regoHandler) evalQuery(q string, input interface{}) (rego.ResultSet, error) {
	ctx := context.Background()

	query, err := rh.rego.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	return query.Eval(ctx, rego.EvalInput(input))
}

// Github functions

func makeIssueComment(violations []interface{}) *github.IssueComment {
	lines := []string{
		"Some policy checks failed, please fix them and then re-run `atlantis plan`:",
		"",
		"```txt",
	}

	for _, elem := range violations {
		v := elem.(map[string]interface{})
		lines = append(lines, fmt.Sprintf("%v: %v", v["addr"], v["msg"]))
	}

	lines = append(lines, "```")

	return &github.IssueComment{
		Body: github.String(strings.Join(lines, "\n")),
	}
}

func makeRepoStatus(pass bool) *github.RepoStatus {
	state := "failure"
	description := "There are some resource policy violations. Please fix them and re-run `atlantis-plan`."
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

func (rh *regoHandler) updateGithubStatus(rs rego.ResultSet, prInfo *prInfo) error {
	pass := rs[0].Bindings["pass"].(bool)
	violations := rs[0].Bindings["violations"].([]interface{})

	log.Printf("Pass: %t (%d violations)\n", pass, len(violations))

	ctx := context.Background()

	pr, _, err := rh.gh.PullRequests.Get(ctx, prInfo.Owner, prInfo.Name, prInfo.PullNum)
	if err != nil {
		return fmt.Errorf("error getting PR info: %v", err)
	}

	ref := pr.GetHead().SHA

	if !pass {
		comment := makeIssueComment(violations)
		_, _, err := rh.gh.Issues.CreateComment(ctx, prInfo.Owner, prInfo.Name, prInfo.PullNum, comment)
		if err != nil {
			log.Println(err)
			return fmt.Errorf("error adding issue comment")
		}
	}

	status := makeRepoStatus(pass)
	_, _, err = rh.gh.Repositories.CreateStatus(ctx, prInfo.Owner, prInfo.Name, *ref, status)
	if err != nil {
		log.Println(err)
		return fmt.Errorf("error updating PR status")
	}

	return nil
}
