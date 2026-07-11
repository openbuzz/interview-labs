#!/usr/bin/env bats

load 'helpers'

@test "list aligns long slugs and dashes empty IPs" {
  seed_session "extraordinarily-photogenic-mongoose-x9k2" "203.0.113.7" \
    "ready" 90
  seed_session "tiny-ant-1a2b" "" "failed" 5

  tmux_start "list" "$(tui_cmd 'interview list')"
  tmux_wait_done

  tmux_snapshot "list"
  [[ "$(tmux_exit_code)" == "0" ]]
}
