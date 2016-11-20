FTL - Faster than Light
======

Amazon AWS based deploy system

Overview
-----
The most naive and bare-bones of deployment systems is to use SSH to copy files
around to multiple nodes. This can be easily implemented in something like
'Fabric'. You could call this a 'push' based deployment system.

A big issue with this style is dealing with systems that may not be
running when the deployment takes place. You have to start the system, then do
a deployment to get the software on the new machine.

FTL provides a deployment system that isn't much more complex than a simple
push model, but uses S3 to get the characteristics of a pull system. FTL also
provides a reasonable amount of framework for switching between revisions of
your software and automating installation and setup of your software.

Simple Example Usage
----

Create the new revision

    $ tar -czf my_site.tgz src/*
    $ ftl spool my_site.tgz
    my_site.054aR4G0L

Use SSH to kick all your servers to get the new revision

    $ <ssh all> ftl sync

Activate the new revision

    $ <ssh all> ftl jump my_site.054aR4G0L
    $ ftl jump --remote my_site.054aR4G0L


Commands
----

    ftl spool <package_name>.tar.gz    # Upload new revision
    ftl list                           # List available packages
    ftl list <package name>            # List available revisions for the package
    ftl list --remote <package name>   # List available revisions for the package on the remote repository (S3)
    ftl sync                           # Check S3 for new stuff to do (new revisions, remove revisions, bless)
    ftl jump <rev name>                # Activate the specified revision
    ftl jump-back <package name>       # Activiate the previous revision
    ftl jump --remote <rev name>       # Mark the revision blessed in S3 (only needs to be done by one node)
    ftl purge --remote <rev name>      # Remove the specified revision.


Installation and Setup
-----

Standard GO application installation:

    $ go get github.com/rhettg/ftl
    $ go install github.com/rhettg/ftl/main.go

FTL has no config files. It's entirely environment variable based.

You'll need on each machine whenever FTL is run:

    FTL_BUCKET=<S3 Bucket to use>
    FTL_ROOT=<deployment directory>

This deployment directory will need a subdirectory for whatever packages your
system should care about. So if you have a package amed `my_site`, your
deployment directory might look like:

    /var/opt/deploy/
        my_site/
           .. empty ..

Your current version of the package will be accesssed as:

    /var/opt/deploy/my_site/current/<file name>

FTL needs access to the configured S3 Bucket. FTL can make use of AWS
credentials similarly to standard command line AWS tools. You can rely on
instance IAM profiles or provide environment variables like:

    AWS_SECRET_ACCESS_KEY=<secret>
    AWS_ACCESS_KEY_ID=<key>

If your `FTL_BUCKET` is not in the the standard us-east region, you can specify the region with:

    AWS_DEFAULT_REGION=us-west-2

Deployment Package
-----

Package names are inferred from the spooled file name, extracting the string up to the first `.`.

The file can be of any format, but there is special handling for `.tar`, `.gz`
and `.tgz` files.  For these, we'll unzip and/or untar them into the revision
directory for you.

In addition, if specially named scripts are provided in the tar file, we'll run
them at the specified steps in the deployment.

  * `post-sync.sh`
  * `pre-jump.sh`
  * `post-jump.sh`
  * `un-jump.sh`
  * `pre-remove.sh`

If any of the `pre` scripts exit with an error, the step will not complete. If
any script exits with an error, the FTL command will always exit with an error.

Deploy Directory Layout
----

    .lock                                # Lock file to syncronize processes (cron vs. manual)
    <project>/
              current/                   # Symlink to current revision
              revs/
                   201303057568Wq/       # Specific revision
                        ftl/post-spool   # Script to be executed after download
                        ftl/pre-jump     # Script to be executed before bless
                        ftl/post-jump    # Script executed after bless
                        ftl/un-jump      # Script executed before un-blessing
                        ....             # More package data

S3 Layout
-----
    <package_name>.fhsdjf.tar.gz   # Specific revision
    <package_name>.rev             # Active revision name


Development
------

There are two types of testing available:

  * Unit tests provided via standard `go test`
	* Integration tests written in bash as scripts in the `tests` directory

To run integration tests, you'll need a real live S3 bucket. This bucket must
be empty. You can run all the tests via the Makefile:

    $ make full-test

Todo
------

  1. Environment variables for package scripts
  1. Fixup logging
  1. Output capture and annotate rather than echo for package scripts
  1. Lock file

License
-------
ISC

Copyright (c) 2014, Rhett Garber <rhettg@gmail.com>

Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
