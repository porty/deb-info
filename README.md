# Deb Info

Prints information about a specified Debian package

## Building

```
go build .
```

## Installing

```
go install .
```

## Running

Run `deb-info` with the name of the Debian archive to parse.
If filename not specified, will read from standard input.

`deb-info` will first print out the contents of the `control` file from the control archive, followed by a file listing of the data archive.

Example:

```
$ deb-info pgq-14_3.4.1-0_amd64.deb
Package: pgq-14
Version: 3.4.1-0
License: unknown
Vendor: none
Architecture: amd64
Maintainer: <@docker-desktop>
Installed-Size: 1533
Depends: postgresql-14.1 | postgresql-14.2
Section: default
Priority: extra
Homepage: https://wiki.postgresql.org/wiki/PGQ_Tutorial
Description: Generic Queue for PostgreSQL

Name                                                                                                 Mode          Size          MIME
./                                                                                                   drwxr-xr-x                0 <DIR>
./usr/                                                                                               drwxr-xr-x                0 <DIR>
./usr/share/                                                                                         drwxr-xr-x                0 <DIR>
./usr/share/postgresql/                                                                              drwxr-xr-x                0 <DIR>
./usr/share/postgresql/14.1/                                                                         drwxr-xr-x                0 <DIR>
./usr/share/postgresql/14.1/contrib/                                                                 drwxr-xr-x                0 <DIR>
./usr/share/postgresql/14.1/contrib/pgq_pl_only.sql                                                  -rw-r--r--           146146 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/newgrants_pgq.sql                                                -rw-r--r--             7328 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/pgq.upgrade.sql                                                  -rw-r--r--           100707 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/pgq_pl_only.upgrade.sql                                          -rw-r--r--           135969 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/oldgrants_pgq.sql                                                -rw-r--r--             5227 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/pgq.sql                                                          -rw-r--r--           110884 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/uninstall_pgq.sql                                                -rw-r--r--               52 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/pgq_triggers.sql                                                 -rw-r--r--             4953 text/plain; charset=utf-8
./usr/share/postgresql/14.1/contrib/pgq_lowlevel.sql                                                 -rw-r--r--             1170 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/                                                               drwxr-xr-x                0 <DIR>
./usr/share/postgresql/14.1/extension/pgq.control                                                    -rw-r--r--              143 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/pgq--3.2.6--3.4.1.sql                                          -rw-r--r--           100707 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/pgq--3.4.1.sql                                                 -rw-r--r--           111411 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/pgq--3.2--3.4.1.sql                                            -rw-r--r--           100707 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/pgq--3.2.3--3.4.1.sql                                          -rw-r--r--           100707 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/pgq--unpackaged--3.4.1.sql                                     -rw-r--r--           101597 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/pgq--3.4--3.4.1.sql                                            -rw-r--r--           100707 text/plain; charset=utf-8
./usr/share/postgresql/14.1/extension/pgq--3.3.1--3.4.1.sql                                          -rw-r--r--           100707 text/plain; charset=utf-8
./usr/share/doc/                                                                                     drwxr-xr-x                0 <DIR>
./usr/share/doc/pgq-14/                                                                              drwxr-xr-x                0 <DIR>
./usr/share/doc/pgq-14/changelog.gz                                                                  -rw-r--r--              140 application/gzip
./usr/lib/                                                                                           drwxr-xr-x                0 <DIR>
./usr/lib/postgresql/                                                                                drwxr-xr-x                0 <DIR>
./usr/lib/postgresql/14.1/                                                                           drwxr-xr-x                0 <DIR>
./usr/lib/postgresql/14.1/lib/                                                                       drwxr-xr-x                0 <DIR>
./usr/lib/postgresql/14.1/lib/pgq_triggers.so                                                        -rwxr-xr-x           276952 application/x-sharedlib
./usr/lib/postgresql/14.1/lib/pgq_lowlevel.so                                                        -rwxr-xr-x            64600 application/x-sharedlib

File count: 20
Directory count: 13
Total file size: 1570814
```

## Future

* allow non-gzip control/data archives
* lint file contents (added .git archives, non-executable binaries, mismatched binary architecture (ARM64 ELF binaries with `amd64` control Arch)
* human-readable file sizes
* more flexible table sizing
