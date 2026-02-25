# Third-Party Software Licenses

This file contains license information for third-party software components used in karta.

For complete license texts, please refer to the source repositories or the LICENSE files in the respective dependency packages.

## Direct Dependencies
{{ range . }} 
### {{ .Name }}
- Name: {{ .Name }}
- Version: {{ .Version }}
- License: {{ .LicenseName }}
- Repository: {{ .LicenseURL }}
{{ end }}

## License Texts

Full license texts for the above dependencies can be found in their respective source repositories or in the vendor directory if vendoring is used. The most common licenses used are:


For detailed license information, please refer to:
- The LICENSE files in each dependency's source repository
- The go.sum file which contains checksums and version information
- The vendor directory (if vendoring is enabled)
