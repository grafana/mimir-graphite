<a href="https://goreportcard.com/report/github.com/grafana/mimir-graphite"><img src="https://goreportcard.com/badge/github.com/grafana/mimir-graphite" alt="Go Report Card" /></a>

# Graphite Whisper Converter

## Summary

Grafana Cloud's Graphite Integration stores Graphite data in a standard Mimir backend by translating traditional Graphite metric data into a form that the Graphite proxy can query.
If you are migrating to Grafana Cloud, you may want to bring your existing archival Graphite data with you, and this Whisper Converter, in concert with `mimirtool`, enables that migration.
You can run this tool yourself with minimal intervention by Grafana Labs.

## Graphite Ecosystem Background

Some background about Graphite is necessary to understand what the tool does and its limitations.

### Storage Backend

There are multiple implementations of Graphite, and they all store data on disk differently.
It is important to know which implentation you're using:

* (Supported) Graphite Web, or sometimes "Classic Graphite": This is the original and official Python implementation of Graphite. It stores data in Whisper files.
* (Maybe supported) Go-Graphite: This is a Go rewrite of Graphite that can use either Whisper (supported) or Clickhouse (not supported) as its storage backend.
* (Not supported) BigGraphite: This is another Python implementation of Graphite that uses Cassandra as a storage backend.

This migration tool **only** supports conversion of Graphite data stored in Whisper files.
If you are using a different form of Graphite, you will have to convert your data to whisper files before you can use this tooling.
Grafana Labs does not support nor have expertise in converting other data formats to Whisper files.

### Untagged versus Tagged metrics

Graphite supports two types of metrics: untagged and tagged.
This conversion tooling only supports untagged metrics.

## Conversion of Whisper to Mimir Blocks

The first major step in the conversion process is converting the Graphite Whisper files to Mimir blocks, and that's what this tool does.

### Disk Usage Reduction

After conversion, the resulting Mimir blocks will probably be much smaller than the incoming Graphite Whisper files.
This is because Whisper files are structured to improve write performance at the expense of disk space.
You should not be concerned that data is missing because the output file size is smaller.

### mimir-whisper-converter usage

The mimir-whisper-converter utility has good documentation when run with --help.
Please reference the built-in documentation for exact usage instructions.

Because archives can be very large, it does file conversion in multiple steps, designed to run separately to reduce memory consumption.

**Note:** mimir-whisper-converter has robust support for resuming partially-completed runs.
As long as the input data has not changed, it is safe to interrupt and restart a conversion, even if it crashed.

#### Step 1 [optional]: Generate the list of files to be processed.

In the case where you have many many thousands of Whisper files, it is useful to pre-generate the list of files to be processed.
Otherwise, a large amount of time will be spent on every command regenerating this list by walking the file system and loading it into memory, and this is slow and can cause out-of-memory crashes.

`mimir-whisper-converter --whisper-directory /opt/graphite/storage/whisper --target-whisper-file /opt/graphite/whisper-files.txt filelist`

#### Step 2 [optional]: Determine the date range to be processed.

The converter needs to know what date range will be covered by the conversion, and this command processes all of the whisper files to determine what this range is.
This process is also slow, so if you already know the date range you can specify it manually.

This command saves the date range to a variable for use in later commands.

`rangeOpts=$(mimir-whisper-converter --whisper-directory /opt/graphite/storage/whisper --target-whisper-file /opt/graphite/whisper-files.txt --quiet daterange)`

#### Step 3: First pass conversion of whisper files to intermediate files.

To prevent out-of-memory errors, the actual conversion of whisper files is done in two passes using a Map/Reduce pattern.
This first pass reshuffles the data so it can be easily converted in the second pass without exploding memory.
This example command uses the rangeOpts variable from the previous step.
This will be the slower of the two passes.

`mimir-whisper-converter --whisper-directory /opt/graphite/storage/whisper $rangeOpts --intermediate-directory /tmp/intermediate pass1`

#### Step 4: Second pass conversion of intermediate files to Mimir blocks.

The second pass should run much more quickly and generates the finished Mimir block files.

`mimir-whisper-converter --intermediate-directory /tmp/intermediate --blocks-directory /opt/mimir/blocks $rangeOpts pass2`

## Uploading Mimir blocks to Grafana

Once the archival data is converted to Mimir blocks, it can be uploaded to Grafana using mimirtool using the "backfill" command.
Note that this command will not *overwrite* or *replace* existing data.
If data already exists for the specified timestamps, you will end up with *two copies* of data -- the old data you thought you were replacing, and these new blocks.
If you need help replacing data rather than just starting from an empty database, please contact Grafana directly.

### Block Validation

By default, once the blocks are uploaded, Mimir will perform a lengthy and slow validation of the incoming data to ensure it is error-free.
While this validation is normally recommended, it may slow down the migration too much.
We have yet to have a situation where mimir-whisper-converter generated invalid data, so it is ok to disable this validation.
This must be done internally at Grafana before you initiate the backfill command.

### No Easy Access via Prometheus or PromQL

Although the Graphite data has been converted to Mimir blocks, the data will not be easily queryable via PromQL or through the Prometheus stack on Grafana Cloud.
This is due to the technical implementation of Graphite on Grafana and there are no plans to support this access method in the future.

### Basic Usage
Here is an example invocation of the backfill command:

`mimirtool backfill --address=https://prometheus-prod-XX-prod-us-central-0.grafana.net/ --id=[Graphite Instance ID] --key="<redacted>" /opt/mimir/blocks/*`

**Important Note:**

Notice that while the tool is uploading to the Prometheus DNS Endpoint, we specify the id of the **Graphite** Instance.
This is counter-intuitive but necessary for the data to show up in Grafana Cloud correctly.

If you try to upload data to the Graphite DNS endpoint, you will get an error, because that endpoint is a proxy, not a storage backend.
If you try to upload the data using the Prometheus Instance ID, the command will succeed, but the data will not be in the correct stack and will not be accessible.

Further note that the Instance IDs for Prometheus and Graphite are different by only one digit, so this is an easy mistake to make!

### Verification

Once mimirtool is done uploading, there may be a delay before data appears in Grafana.
But when it does, the data should be available in the Explore mode using the Graphite backend.