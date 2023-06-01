/*
Package ddns provides functions useful for updating Dynamic DNS records.

Usage will always start with [ddns.New],
which returns the DDNSClient implementation.
New requires a domain name which will be updated and a [Provider] implementation for a DNS provider.
Additional client configuration options are listed in the docs for New.
*/
package ddns
