#!/usr/bin/env python3
"""Point the ZTE MC888 router's DNS at Pi-hole, or hand DNS back to the router.

The router gives no way to choose the DNS server its DHCP hands out: clients
always get the router itself (192.168.0.1), which forwards to the mobile
network's DNS. The one DNS setting it has is per APN profile, and only takes
effect in manual APN mode. So:

  enable  copies the automatic APN profile (operator, APN, login) into a manual
          profile named PROFILE_NAME, with Pi-hole as its only DNS server, and
          makes it the active one. The router then forwards every client's
          queries to Pi-hole. Refuses if the router is already in manual APN
          mode with some other profile, so undo always means "back to auto".
  undo    switches the APN back to automatic and deletes that profile, so the
          router manages DNS fully again. Safe to run at any time, repeatedly.
  status  shows the APN mode, DNS settings and connection state.

The router only changes the active APN while mobile data is disconnected, so
enable and undo drop the internet for roughly 10-30s. They only need the LAN,
so undo works even when the internet is down.

Settings come from environment variables, falling back to router/.env next
to this script (gitignored; copy router/.env.example): ROUTER_ADDR (default
192.168.0.1), ROUTER_USER (default user), ROUTER_PASSWORD, PIHOLE_DNS (the
Pi-hole host's address, needed by enable).
"""
import hashlib
import http.cookiejar
import json
import os
import pathlib
import random
import socket
import struct
import sys
import time
import urllib.parse
import urllib.request

PROFILE_NAME = "Pi-hole DNS"
MAX_PROFILES = 10  # the web UI's maxApnNumber
TIMEOUT = 10


def load_env():
    """Read KEY=value lines from router/.env; the real environment wins."""
    env = {}
    path = pathlib.Path(__file__).resolve().parent / ".env"
    if path.exists():
        for line in path.read_text().splitlines():
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env[k.strip()] = v.strip()
    env.update(os.environ)
    return env


def sha256(s):
    return hashlib.sha256(s.encode()).hexdigest().upper()


class Router:
    def __init__(self, addr, user, password):
        self.base = "http://" + addr
        self.user, self.password = user, password
        self.headers = {"Referer": self.base + "/index.html", "X-Requested-With": "XMLHttpRequest"}
        self.opener = urllib.request.build_opener(
            urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def _open(self, path, data=None):
        req = urllib.request.Request(self.base + path, data=data, headers=self.headers)
        with self.opener.open(req, timeout=TIMEOUT) as r:
            body = r.read().decode(errors="replace")
        return json.loads(body) if body.strip() else {}

    def get(self, *names):
        q = {"isTest": "false", "cmd": ",".join(names)}
        if len(names) > 1:
            q["multi_data"] = "1"
        return self._open("/goform/goform_get_cmd_process?" + urllib.parse.urlencode(q))

    def login(self):
        # Password is SHA256(SHA256(password) + LD), LD a per-attempt nonce.
        fails = self.get("psw_fail_num_str", "login_lock_time")
        if fails.get("psw_fail_num_str") == "0":
            sys.exit(f"router: login locked, try again in {fails.get('login_lock_time')}s")
        ld = self.get("LD")["LD"]
        r = self._post({"goformId": "LOGIN", "user": self.user,
                        "password": sha256(sha256(self.password) + ld)}, sign=False)
        if r.get("result") != "0":
            left = self.get("psw_fail_num_str").get("psw_fail_num_str")
            sys.exit(f"router: login failed ({r}); {left} attempts left before lockout")

    def _post(self, fields, sign=True):
        fields = {"isTest": "false", **fields}
        if sign:
            # Every set except LOGIN carries AD = SHA256(SHA256(rd0 + rd1) + RD):
            # rd0/rd1 are the firmware versions, RD a per-request nonce.
            v = self.get("wa_inner_version", "cr_version")
            rd = v.get("wa_inner_version", "") + v.get("cr_version", "")
            fields["AD"] = sha256(sha256(rd) + self.get("RD")["RD"])
        return self._open("/goform/goform_set_cmd_process",
                          urllib.parse.urlencode(fields).encode())

    def set(self, fields):
        r = self._post(fields)
        if r.get("result") != "success":
            sys.exit(f"router: {fields.get('goformId')} failed: {r}")

    def apn_state(self):
        names = ["apn_mode", "Current_index", "m_profile_name", "dns_mode", "prefer_dns_manual",
                 "standby_dns_manual", "prefer_dns_auto", "standby_dns_auto", "apn_auto_config",
                 "ppp_status", "wan_ipaddr"]
        names += [f"APN_config{i}" for i in range(MAX_PROFILES)]
        return self.get(*names)

    def wait_ppp(self, want, seconds=90):
        end = time.time() + seconds
        while time.time() < end:
            status = self.get("ppp_status").get("ppp_status", "")
            connected = status.endswith("connected") and status != "ppp_disconnected"
            if (want == "down" and status == "ppp_disconnected") or (want == "up" and connected):
                return status
            time.sleep(2)
        return None

    def disconnect(self):
        print("router: disconnecting mobile data")
        self.set({"goformId": "DISCONNECT_NETWORK", "notCallback": "true"})
        if not self.wait_ppp("down", 30):
            print("router: still not disconnected after 30s, carrying on", file=sys.stderr)

    def connect(self):
        print("router: reconnecting mobile data")
        self.set({"goformId": "CONNECT_NETWORK", "notCallback": "true"})
        status = self.wait_ppp("up")
        print(f"router: {status or 'NOT connected after 90s -- check the router, or run undo'}")
        return bool(status)


def profiles(state):
    """Manual APN profiles as {index: fields}; fields are split on the '($)' separator."""
    out = {}
    for i in range(MAX_PROFILES):
        cfg = state.get(f"APN_config{i}", "")
        if cfg:
            out[i] = cfg.split("($)")
    return out


def find_profile(state):
    for i, f in profiles(state).items():
        if f[0] == PROFILE_NAME:
            return i
    return None


def dns_query(server, name, timeout=3):
    """Ask server for name's A record; returns the IPv4 answers (raises on no reply)."""
    qid = random.randint(0, 0xFFFF)
    q = struct.pack(">HHHHHH", qid, 0x0100, 1, 0, 0, 0)
    q += b"".join(bytes([len(p)]) + p.encode() for p in name.split(".")) + b"\0"
    q += struct.pack(">HH", 1, 1)
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
        s.settimeout(timeout)
        s.sendto(q, (server, 53))
        resp = s.recv(2048)
    ancount = struct.unpack(">H", resp[6:8])[0]
    i = len(q)
    ips = []
    for _ in range(ancount):
        i += 2 if resp[i] & 0xC0 == 0xC0 else resp.index(b"\0", i) + 1 - i
        rtype, _, _, rdlen = struct.unpack(">HHIH", resp[i:i + 10])
        i += 10
        if rtype == 1 and rdlen == 4:
            ips.append(socket.inet_ntoa(resp[i:i + 4]))
        i += rdlen
    return ips


def check_dns(server, label):
    """Print whether server resolves a normal name and blocks an ad domain."""
    try:
        ok = dns_query(server, "bbc.co.uk")
        ad = dns_query(server, "doubleclick.net")
    except OSError as e:
        print(f"dns: {label} ({server}) not answering: {e}")
        return False, False
    blocked = ad in ([], ["0.0.0.0"])
    print(f"dns: {label} ({server}) resolves bbc.co.uk: {'yes' if ok else 'NO'}; "
          f"doubleclick.net {'blocked' if blocked else 'not blocked'} {ad}")
    return bool(ok), blocked


def cmd_status(r, env):
    s = r.apn_state()
    print(f"apn mode:     {s['apn_mode']}" + (f" (profile {s['Current_index']})" if s["apn_mode"] == "manual" else
                                              f" ({s['m_profile_name']})"))
    print(f"dns mode:     {s['dns_mode']}  manual: {s['prefer_dns_manual'] or '-'} / {s['standby_dns_manual'] or '-'}"
          f"  operator: {s['prefer_dns_auto'] or '-'} / {s['standby_dns_auto'] or '-'}")
    print(f"mobile data:  {s['ppp_status']}  wan {s['wan_ipaddr'] or '-'}")
    for i, f in profiles(s).items():
        dns = f"dns {f[10]} {f[11]} {f[12]}".rstrip() if len(f) > 12 else ""
        print(f"profile {i}:    {f[0]}  apn={f[1]}  {dns}")
    check_dns(env.get("ROUTER_ADDR", "192.168.0.1"), "router")


def cmd_enable(r, env):
    pihole = env.get("PIHOLE_DNS")
    if not pihole:
        sys.exit("enable: set PIHOLE_DNS (the Pi-hole host's address) in router/.env")
    ok, blocked = check_dns(pihole, "pi-hole")
    if not ok:
        sys.exit("enable: Pi-hole isn't resolving, not pointing the router at it")

    s = r.apn_state()
    ours = find_profile(s)
    if s["apn_mode"] == "manual" and (ours is None or s["Current_index"] != str(ours)):
        sys.exit("enable: router is already in manual APN mode with another profile; "
                 "not touching it (undo only knows how to go back to auto)")

    # Copy the operator's automatic profile: name($)apn($)..($)dial($)auth($)user($)pass($)pdp_type
    auto = s["apn_auto_config"].split("($)")
    apn, auth, user, password, pdp = auto[1], auto[4].lower(), auto[5], auto[6], auto[7]
    if not apn:
        sys.exit(f"enable: can't read the automatic APN profile: {s['apn_auto_config']!r}")
    index = ours if ours is not None else len(profiles(s))
    if index >= MAX_PROFILES:
        sys.exit("enable: no free APN profile slot")
    print(f"router: saving profile {index} '{PROFILE_NAME}' (apn {apn}, {pdp}) with DNS {pihole}")
    r.set({"goformId": "APN_PROC_EX", "apn_action": "save", "apn_mode": "manual",
           "profile_name": PROFILE_NAME, "wan_dial": "*99#", "apn_select": "manual",
           "pdp_type": pdp, "pdp_select": "auto", "pdp_addr": "", "index": str(index),
           "wan_apn": apn, "ppp_auth_mode": auth, "ppp_username": user, "ppp_passwd": password,
           "dns_mode": "manual", "prefer_dns_manual": pihole, "standby_dns_manual": "",
           "ipv6_wan_apn": apn, "ipv6_ppp_auth_mode": auth, "ipv6_ppp_username": user,
           "ipv6_ppp_passwd": password, "ipv6_dns_mode": "auto",
           "ipv6_prefer_dns_manual": "", "ipv6_standby_dns_manual": ""})

    r.disconnect()
    print(f"router: making profile {index} active (manual APN mode)")
    r.set({"goformId": "APN_PROC_EX", "apn_mode": "manual", "apn_action": "set_default",
           "set_default_flag": "1", "pdp_type": pdp, "index": str(index)})
    up = r.connect()

    s = r.apn_state()
    print(f"router: apn mode {s['apn_mode']}, dns mode {s['dns_mode']}, dns {s['prefer_dns_manual']}")
    _, blocked = check_dns(env.get("ROUTER_ADDR", "192.168.0.1"), "router")
    if not up or s["dns_mode"] != "manual" or not blocked:
        print("enable: not fully working -- run `router/pihole-dns.py undo` to put it back", file=sys.stderr)
        sys.exit(1)
    print("enable: done, the router now forwards DNS to Pi-hole")


def cmd_undo(r, env):
    s = r.apn_state()
    if s["apn_mode"] != "auto" or s["ppp_status"] != "ppp_connected":
        r.disconnect()
        print("router: switching APN back to automatic")
        r.set({"goformId": "APN_PROC_EX", "apn_mode": "auto"})
        r.connect()
    else:
        print("router: APN already automatic")
    ours = find_profile(r.apn_state())
    if ours is not None:
        print(f"router: deleting profile {ours} '{PROFILE_NAME}'")
        r.set({"goformId": "APN_PROC_EX", "apn_action": "delete", "apn_mode": "manual", "index": str(ours)})
    cmd_status(r, env)


def main():
    cmds = {"status": cmd_status, "enable": cmd_enable, "undo": cmd_undo}
    if len(sys.argv) != 2 or sys.argv[1] not in cmds:
        sys.exit(f"usage: {sys.argv[0]} status|enable|undo")
    env = load_env()
    if not env.get("ROUTER_PASSWORD"):
        sys.exit("set ROUTER_PASSWORD in router/.env (or the environment)")
    r = Router(env.get("ROUTER_ADDR", "192.168.0.1"), env.get("ROUTER_USER", "user"), env["ROUTER_PASSWORD"])
    r.login()
    cmds[sys.argv[1]](r, env)


if __name__ == "__main__":
    main()
