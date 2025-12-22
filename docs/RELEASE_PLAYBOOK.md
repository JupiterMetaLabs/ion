# Release & Operations Playbook

This document outlines the steps for publishing, maintaining, and governing the `ion` library.

## 1. Publishing to Go Package Site
The Go package site (pkg.go.dev) automatically crawls modules, but you can trigger it manually to make a new version available immediately.

1.  **Visit this URL** to trigger a fetch:
    ```
    https://pkg.go.dev/github.com/JupiterMetaLabs/ion@v0.1.0
    ```
    *(Replace `v0.1.0` with your latest tag)*
2.  Click "Request this version" if prompted.
3.  Wait ~5 minutes for the documentation to render.

## 2. GitHub Releases
Turn your Git Tag into a proper GitHub Release to communicate changes to the team.

1.  Go to `https://github.com/JupiterMetaLabs/ion/releases/new`.
2.  **Choose Tag**: Select `v0.1.0`.
3.  **Release Title**: `v0.1.0 - Enterprise Refactor`.
4.  **Description**: Click the **"Generate release notes"** button. GitHub will auto-summarize your PRs.
5.  Click **Publish release**.

## 3. GitHub Ruleset Configuration (Governance)

To ensure the `main` branch remains stable while giving you control, use the **Rulesets** feature (Settings -> Rules -> Rulesets).

### Recommended Settings

1.  **Target branches**: Click "Add target" -> Include default branch (`main`).
2.  **Bypass list** (The "God Mode" setting):
    *   *Goal*: Allow you to merge without waiting for others.
    *   *Action*: Click "Add bypass" -> Select **"Repository admin"** (or your specific username).
    *   *Permission*: Set to **"Always allow"**.
    *   *Effect*: You can push directly or merge PRs without approvals, while everyone else is restricted.

3.  **Branch rules** (Check these specific boxes):
    *   [x] **Restrict deletions**: Prevents accidental deletion of `main`.
    *   [x] **Require a pull request before merging**:
        *   *Required approvals*: **1**.
        *   *Dismiss stale pull request approvals*: **Checked** (Recommended).
    *   [x] **Require status checks to pass**:
        *   Click "Add check" -> Search for `test` (or `Build and Test`).
        *   *Troubleshooting*: If the list is empty, wait for the GitHub Action to finish running on your latest push. GitHub only lists checks that have reported a status at least once.
    *   [x] **Block force pushes**: Critical to prevent history rewrites.

## 4. Sharing with Teams
Post this announcement to your engineering Slack/Teams channel:

> **🚀 Announcement: New Logging Standard (`ion` v0.1.0)**
>
> Hi Team,
>
> We have published `ion`, our new enterprise-grade structured logger tailored for JupiterMeta's blockchain services.
>
> **Why use it?**
> *   🚀 **Zero-Allocation**: Optimized for high-throughput hot paths.
> *   🔭 **OTEL Native**: Automatic trace propagation and export.
> *   🛡️ **Safe**: Graceful shutdown and resource management.
>
> **Get Started**:
> `go get github.com/JupiterMetaLabs/ion@v0.1.0`
>
> **Documentation**:
> Check out the [README](https://github.com/JupiterMetaLabs/ion) for examples and best practices.
