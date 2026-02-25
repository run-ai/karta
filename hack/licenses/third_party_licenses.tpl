# Third-Party Software Licenses

This file contains license information for third-party software components used in karta.

For complete license texts, please refer to the source repositories or the LICENSE files in the respective dependency packages.

## Dependencies
{{ range . }} 
### {{ .Name }}
- Name: {{ .Name }}
- Version: {{ .Version }}
- License: [{{ .LicenseName }}]({{ .LicenseURL }})
{{ end }}

## License Texts

Full license texts for the above dependencies can be found in their respective source repositories or in the vendor directory if vendoring is used. The most common licenses used are:

- Apache-2.0: See LICENSE file in this repository for the full Apache License 2.0 text
- MIT: See individual dependency repositories for MIT license text
- BSD-3-Clause: See individual dependency repositories for BSD 3-Clause license text
- ISC: See individual dependency repositories for ISC license text
- Unicode-DFS: See individual dependency repositories for Unicode License Agreement text

For detailed license information, please refer to:
- The LICENSE files in each dependency's source repository
- The go.sum file which contains checksums and version information
- The vendor directory (if vendoring is enabled)
