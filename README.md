FTL - Faster than Light
======

Amazon AWS based deploy system

Overview
-----
Combination of push/pull system. Backed by S3.

Command line tool to push new revision to S3.

Tool on remote systems to poll for new revisions.

Some process to kick remote systems to update.

New revisions + bless. We have to coordinate the bless part.
Collecting revisions just means syncronizing directories. Bless means switching a symlink / executing post-install

Revision names should be time sorted.

Deploy Directory Layout
----

    <project>/
              revs/
                   a93jfhsdjf/         # Specific revision
                        post-sync.sh   # Script to be executed after download
                        pre-jump.sh   # Script to be executed before bless
                        post-jump.sh  # Script executed after bless
                        un-jump.sh    # Script executed before un-blessing
              current/                 # Symlink to current revision
              .lock                    # Lock file to syncronize processes (cron vs. manual)

S3 Layout
-----
    <project>/
              revs/
                   a93jfhsdjf.tar.gz   # Specific revision
              current                  # File containing blessed revision name

Deploy Process
-----

  * daemon polls S3 for new revisions
  * download new revision, run pre-bless.sh
  * daemon polls 'current' file on S3
  * when new revision is blessed, change symlink, run post-bless

There are also ways to short-circuit polling. SSH into machine, and then run commands directly.


Commands
----

    ftl spool <filename.tar.gz>   # Upload new revision
    ftl list                      # List available revisions
    ftl sync                      # Check S3 for new stuff to do (new revisions, remove revisions, bless)
    ftl jump <rev name>           # Bless the specified revision
    ftl jump --master <rev name>  # Mark the revision blessed in S3 (only needs to be done by one node)
