package tlsutils

import (
	"crypto/x509"
	"net/url"

	slices2 "github.com/ing-bank/golibs/pkg/slices"
)

// ExtractCommonNames returns all non-empty CommonNames from the given certificates.
func ExtractCommonNames(certs []*x509.Certificate) []string {
	return slices2.Filter(
		slices2.Transform(certs, func(cert *x509.Certificate) string {
			return cert.Subject.CommonName
		}),
		func(cn string) bool { return cn != "" },
	)
}

// ExtractDNSNames returns all non-empty DNSNames from the given certificates.
func ExtractDNSNames(certs []*x509.Certificate) []string {
	return slices2.Unique(slices2.Filter(
		slices2.FlatMap(certs, func(cert *x509.Certificate) []string {
			return cert.DNSNames
		}),
		func(dns string) bool { return dns != "" },
	))
}

// ExtractCommonNamesAndDNSNames returns all non-empty CommonNames and DNSNames from the given certificates (combined).
func ExtractCommonNamesAndDNSNames(certs []*x509.Certificate) []string {
	return slices2.Concat(
		ExtractCommonNames(certs),
		ExtractDNSNames(certs),
	)
}

// ExtractCommonNamesAndDNSNamesSeparate returns CommonNames and DNSNames as separate slices.
func ExtractCommonNamesAndDNSNamesSeparate(certs []*x509.Certificate) ([]string, []string) {
	return ExtractCommonNames(certs), ExtractDNSNames(certs)
}

// ExtractURLs returns all non-empty URIs from the given certificates.
func ExtractURLs(certs []*x509.Certificate) []*url.URL {
	return slices2.FilterEmpty(
		slices2.FlatMap(certs, func(cert *x509.Certificate) []*url.URL {
			return cert.URIs
		}),
	)
}

// ExtractLeafCommonName returns the subject CommonName of the leaf (first) certificate only.
// The subject CommonName of the leaf (first) certificate is the primary identity field in the subject
// of the client or server certificate presented during a TLS handshake.
// The "leaf" certificate is the end-entity certificate (not an intermediate or root CA) and
// is always the first in the certificate chain. Its CommonName typically represents the hostname,
// service, or user being authenticated.
func ExtractLeafCommonName(certs []*x509.Certificate) string {
	if len(certs) == 0 {
		return ""
	}
	return certs[0].Subject.CommonName
}
