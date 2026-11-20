## Network is egress walled, allowlist-only

All egress is forced through a proxy that **denies by default**, EXCEPT loopback (`localhost`, `127.0.0.1`, `::1`) via `no_proxy`.

Allow domains include package registries, mise, GitHub, canonical man text, etc. Run `ccboxtools allowlist` for the current live bare-domain list. When encountering network errors, do not work around the firewall, instead check against the list and tell me what domain failed.