#!/sbin/openrc-run

name=$RC_SVCNAME
description="Ignis Agent"
supervisor="supervise-daemon"
command="/usr/local/bin/agent"
pidfile="/run/agent.pid"
command_user="ignis:ignis"

depend() {
	after net
}
