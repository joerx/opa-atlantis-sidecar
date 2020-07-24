# OPA Atlantis Sidecar

Small OPA integration to be used with [Atlantis](https://www.runatlantis.io) as a sidecar or as part of other Terraform CI pipelines. Exposes a HTTP endpoint that validates a JSON plan and updates a VCS pull request status depending on the result.

_Note: this is only PoC at this stage - nothing really works and it's terrible code style._

![Demo](./docs/media/demo.png)

## Usage

- Follow https://www.runatlantis.io/guide/testing-locally.html#testing-locally to set up the webhook for local testing
- Create a copy of `.env.example` and change the values according to your setup
- Run `docker-compose up`
- Create a PR in a repo of your choice and see what happens
