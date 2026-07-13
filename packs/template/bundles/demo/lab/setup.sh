#!/usr/bin/env bash
# Runs once inside the candidate container after the stack is healthy.
# Env: INTERVIEW_SESSION_ID, INTERVIEW_BUNDLE, INTERVIEW_SCENARIOS,
# INTERVIEW_LAB_DIR. No cloud credentials are available here.

set -e -u -o pipefail

touch "${INTERVIEW_LAB_DIR}/setup-ran"
cat "${INTERVIEW_LAB_DIR}/motd.txt"
