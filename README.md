big-snapshot-builder
--------------------

1. To stress test catalog snapshot/restore:

    # if you want to reset for a brand new test, but this should not be done
    # between data load and the first snapshot restore as part of the test!
    rm -rf data

    # run this in one terminal, or something with roughly equivalent config flags.
    ./start.sh

2. In another terminal, populate the catalog with a bunch of data:

    rm -rf data
    go run *.go catalog

3. Wait until it prints a log line indicating it has loaded at LEAST 20,000 nodes:

    2024-06-12T09:51:08.952-0500 [INFO]  bsb: node progress: current=20000 total=300000

4. Shut down the Consul server.

5. Make a copy of the `data` directory so that the restore can be repeatable without bleedover.

6. Stare at your perf metrics and re-start the `start.sh` script.

7. Pay attention to the consul logs until the following series has been emitted:

    2024-06-12T09:51:19.333-0500 [INFO]  agent.server.raft: starting restore from snapshot: id=3-467342-1718203820769 last-index=467342 last-term=3 size-in-bytes=199325003
    2024-06-12T09:51:29.437-0500 [INFO]  agent.server.raft: snapshot restore progress: id=3-467342-1718203820769 last-index=467342 last-term=3 size-in-bytes=199325003 read-bytes=143491990 percent-complete="71.99%"
    2024-06-12T09:51:36.239-0500 [INFO]  agent.server.raft: restored from snapshot: id=3-467342-1718203820769 last-index=467342 last-term=3 size-in-bytes=199325003

8. Compare the timing between the `"starting snapshot"` and `"restored from snapshot"` log lines.



