#!/bin/sh
# index.sh ROOT DB
# Flat one-filesystem scan of ROOT into a DuckDB file with hardlinks counted once.
# Run as root (pkexec) to see root-only trees such as /var/lib/docker.
set -eu
root=$1
db=$2
err="$db.errors.log"
tsv="$db.tsv"
rm -f "$db" "$db.wal"

# The CSV sniffer reads its input twice, so a pipe will not do: stage a file.
/usr/bin/find "$root" -xdev -printf '%D\t%i\t%n\t%y\t%s\t%b\t%T@\t%p\n' > "$tsv" 2> "$err"

duckdb "$db" -c "
CREATE TABLE files AS
SELECT dev, ino, nlink, type, size, blocks * 512 AS bytes, to_timestamp(mtime) AS mtime, path
FROM read_csv('$tsv', delim='\t', quote='', escape='', ignore_errors=true,
  columns={'dev':'UBIGINT','ino':'UBIGINT','nlink':'INT','type':'VARCHAR','size':'BIGINT',
           'blocks':'BIGINT','mtime':'DOUBLE','path':'VARCHAR'})
QUALIFY nlink = 1 OR row_number() OVER (PARTITION BY dev, ino ORDER BY path) = 1;

-- children of directory r, with recursive size and file count
CREATE MACRO kids(r) AS TABLE
  SELECT split_part(substr(path, length(r) + 2), '/', 1) AS name,
         sum(bytes)::BIGINT AS bytes, format_bytes(sum(bytes)::BIGINT) AS h,
         count(*) FILTER (WHERE type = 'f') AS files
  FROM files
  WHERE starts_with(path, r || '/')
  GROUP BY 1 ORDER BY 2 DESC;

-- size of everything at or under p
CREATE MACRO under(p) AS TABLE
  SELECT sum(bytes)::BIGINT AS bytes, format_bytes(sum(bytes)::BIGINT) AS h,
         count(*) FILTER (WHERE type = 'f') AS files
  FROM files WHERE path = p OR starts_with(path, p || '/');
"

rm -f "$tsv"
echo "errors: $(wc -l < "$err") lines in $err"
duckdb -readonly "$db" -c "SELECT count(*) AS rows, format_bytes(sum(bytes)::BIGINT) AS total FROM files;"
