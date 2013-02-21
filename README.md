FTL - Faster than Light
======

Amazon AWS based deploy system

Overview
-----
The most naive and bare-bones of deployment systems is to use SSH to copy files
around to multiple nodes. This can be easily implemented in something like
'Fabric's is to use SSH to copy files around to multiple nodes. This can be
easily implemented in something like 'Fabric'. This is 'push' based deployment system.

The biggest issue with this system is dealing with systems that may not be
running when the deployment takes place. You have to start the system, then do
a deployment to get the software on the new machine.

FTL provides a deployment system that isn't much more complex than a simple
push model, but uses S3 to get the characteristics of a pull system.

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
    $ ftl jump --master my_site.054aR4G0L


Commands
----

    ftl spool <package_name>.tar.gz    # Upload new revision
    ftl list                           # List available packages
    ftl list <package name>            # List available revisions for the package
    ftl list --master <package name>   # List available revisions for the package on the remote repository (S3)
    ftl sync                           # Check S3 for new stuff to do (new revisions, remove revisions, bless)
    ftl jump <rev name>                # Bless the specified revision
    ftl jump --master <rev name>       # Mark the revision blessed in S3 (only needs to be done by one node)


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

And then similiar to command line EC2 tools, you need your credentials:

    AWS_SECRET_ACCESS_KEY=<secret>
    AWS_ACCESS_KEY_ID=<key>

You'll likely want to setup `ftl sync` to be run via cron or at system startup to ensure your packages are up to date.


Deployment Package
-----

Package names are inferred from the file name that you spool. It's whatever
string up till the first `.` character.

The file can be anything, but there is special handling for `.tar`, `.gz` and `.tgz` files.
For these, we'll unzip and untar them into the revision directory for you.

In addition, if specially named scripts are provided in the tar file, we'll run them at the specified steps in the deployment.

  * `post-sync.sh`
  * `pre-jump.sh`
  * `post-jump.sh`
  * `un-jump.sh`
  * `pre-remove.sh`

If any of the `pre` scripts exit with an error, the step will not complete. If
any script exits with an error, the FTL command will always exit with an error.

Deploy Directory Layout
----

    <project>/
              revs/
                   a93jfhsdjf/        # Specific revision
                        post-sync.sh  # Script to be executed after download
                        pre-jump.sh   # Script to be executed before bless
                        post-jump.sh  # Script executed after bless
                        un-jump.sh    # Script executed before un-blessing
                        ....          # More package data
              current/                # Symlink to current revision
              .lock                   # Lock file to syncronize processes (cron vs. manual)

S3 Layout
-----
    <package_name>.fhsdjf.tar.gz   # Specific revision
    <package_name>.rev             # Active revision name 


Todo
------

  1. Package scripts (pre/post etc)
  1. Fixup logging
  1. Remove older revisions

