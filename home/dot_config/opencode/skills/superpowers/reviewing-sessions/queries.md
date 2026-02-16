# Session Analysis Query Reference

All queries work with both `opencode db "<SQL>" --format json` and `sqlite3`. Append `--format tsv` to opencode db for tabular output.

## 1. Session Browsing

### List recent sessions (current project)
```sql
SELECT id, title, slug,
       datetime(time_created/1000, 'unixepoch', 'localtime') as created,
       summary_files as files, summary_additions as adds, summary_deletions as dels
FROM session
WHERE directory = '/path/to/project'
ORDER BY time_created DESC
LIMIT 20;
```

### Search sessions by title
```sql
SELECT id, title, datetime(time_created/1000, 'unixepoch', 'localtime') as created
FROM session
WHERE title LIKE '%search term%'
ORDER BY time_created DESC;
```

### Sessions in a date range
```sql
SELECT id, title, directory,
       datetime(time_created/1000, 'unixepoch', 'localtime') as created
FROM session
WHERE time_created > strftime('%s', '2025-06-01') * 1000
  AND time_created < strftime('%s', '2025-07-01') * 1000
ORDER BY time_created DESC;
```

### Session message count and duration
```sql
SELECT s.id, s.title,
       count(m.id) as msg_count,
       (max(m.time_created) - min(m.time_created)) / 60000.0 as duration_min
FROM session s
JOIN message m ON m.session_id = s.id
GROUP BY s.id
ORDER BY msg_count DESC
LIMIT 20;
```

## 2. Tool Analysis

### Tool usage frequency (global)
```sql
SELECT json_extract(data, '$.tool') as tool,
       count(*) as total,
       count(CASE WHEN json_extract(data, '$.state.status') = 'error' THEN 1 END) as errors,
       ROUND(100.0 * count(CASE WHEN json_extract(data, '$.state.status') = 'error' THEN 1 END) / count(*), 1) as error_pct
FROM part
WHERE json_extract(data, '$.type') = 'tool'
GROUP BY tool
ORDER BY total DESC;
```

### Tool errors in a specific session
```sql
SELECT json_extract(data, '$.tool') as tool,
       json_extract(data, '$.callID') as call_id,
       substr(json_extract(data, '$.state.output'), 1, 300) as error_output
FROM part
WHERE session_id = '<SESSION_ID>'
  AND json_extract(data, '$.type') = 'tool'
  AND json_extract(data, '$.state.status') = 'error'
ORDER BY time_created;
```

### Most error-prone tools (ranked by error rate, min 10 calls)
```sql
SELECT json_extract(data, '$.tool') as tool,
       count(*) as total,
       count(CASE WHEN json_extract(data, '$.state.status') = 'error' THEN 1 END) as errors,
       ROUND(100.0 * count(CASE WHEN json_extract(data, '$.state.status') = 'error' THEN 1 END) / count(*), 1) as error_pct
FROM part
WHERE json_extract(data, '$.type') = 'tool'
GROUP BY tool
HAVING total >= 10
ORDER BY error_pct DESC;
```

### Tool usage per session (find sessions heavy on a specific tool)
```sql
SELECT p.session_id, s.title,
       count(*) as tool_calls
FROM part p
JOIN session s ON s.id = p.session_id
WHERE json_extract(p.data, '$.tool') = 'bash'
  AND json_extract(p.data, '$.type') = 'tool'
GROUP BY p.session_id
ORDER BY tool_calls DESC
LIMIT 15;
```

### Skill loading frequency (which skills are loaded most)
```sql
SELECT json_extract(data, '$.state.input.name') as skill_name,
       count(*) as loads
FROM part
WHERE json_extract(data, '$.type') = 'tool'
  AND json_extract(data, '$.tool') = 'skill'
GROUP BY skill_name
ORDER BY loads DESC;
```

## 3. Token & Cost Analysis

### Token usage per session
```sql
SELECT s.id, s.title,
       SUM(json_extract(m.data, '$.tokens.input')) as input_tokens,
       SUM(json_extract(m.data, '$.tokens.output')) as output_tokens,
       SUM(json_extract(m.data, '$.tokens.input')) + SUM(json_extract(m.data, '$.tokens.output')) as total_tokens
FROM session s
JOIN message m ON m.session_id = s.id
WHERE json_extract(m.data, '$.role') = 'assistant'
GROUP BY s.id
ORDER BY total_tokens DESC
LIMIT 20;
```

### Token usage by model
```sql
SELECT json_extract(data, '$.modelID') as model,
       count(*) as messages,
       SUM(json_extract(data, '$.tokens.input')) as input_tokens,
       SUM(json_extract(data, '$.tokens.output')) as output_tokens
FROM message
WHERE json_extract(data, '$.role') = 'assistant'
GROUP BY model
ORDER BY output_tokens DESC;
```

### Highest token sessions in the last N days
```sql
SELECT s.id, s.title, s.directory,
       datetime(s.time_created/1000, 'unixepoch', 'localtime') as created,
       SUM(json_extract(m.data, '$.tokens.input')) + SUM(json_extract(m.data, '$.tokens.output')) as total_tokens
FROM session s
JOIN message m ON m.session_id = s.id
WHERE json_extract(m.data, '$.role') = 'assistant'
  AND s.time_created > strftime('%s', 'now', '-7 days') * 1000
GROUP BY s.id
ORDER BY total_tokens DESC
LIMIT 10;
```

### Agent type distribution
```sql
SELECT json_extract(data, '$.agent') as agent,
       count(*) as messages,
       SUM(json_extract(data, '$.tokens.output')) as output_tokens
FROM message
WHERE json_extract(data, '$.role') = 'assistant'
GROUP BY agent
ORDER BY output_tokens DESC;
```

## 4. Session Lineage

### Root sessions vs forks
```sql
SELECT
  count(*) as total,
  count(CASE WHEN parent_id IS NULL THEN 1 END) as root_sessions,
  count(CASE WHEN parent_id IS NOT NULL THEN 1 END) as forked_sessions,
  ROUND(100.0 * count(CASE WHEN parent_id IS NOT NULL THEN 1 END) / count(*), 1) as fork_pct
FROM session;
```

### Sessions with most forks (subagent-heavy)
```sql
SELECT parent.id, parent.title,
       count(child.id) as fork_count,
       datetime(parent.time_created/1000, 'unixepoch', 'localtime') as created
FROM session parent
JOIN session child ON child.parent_id = parent.id
GROUP BY parent.id
ORDER BY fork_count DESC
LIMIT 15;
```

### Fork chain depth (find deeply nested subagent sessions)
```sql
WITH RECURSIVE chain AS (
  SELECT id, parent_id, title, 1 as depth
  FROM session WHERE parent_id IS NOT NULL
  UNION ALL
  SELECT c.id, s.parent_id, c.title, c.depth + 1
  FROM chain c
  JOIN session s ON s.id = c.parent_id
  WHERE s.parent_id IS NOT NULL
)
SELECT id, title, max(depth) as max_depth
FROM chain
GROUP BY id
ORDER BY max_depth DESC
LIMIT 10;
```

### Trace a session's full lineage (parent chain)
```sql
WITH RECURSIVE ancestors AS (
  SELECT id, parent_id, title, 0 as depth
  FROM session WHERE id = '<SESSION_ID>'
  UNION ALL
  SELECT s.id, s.parent_id, s.title, a.depth + 1
  FROM ancestors a
  JOIN session s ON s.id = a.parent_id
)
SELECT depth, id, title FROM ancestors ORDER BY depth;
```

## 5. Workflow Patterns

### Compaction events (sessions that got too long)
```sql
SELECT p.session_id, s.title,
       count(*) as compactions,
       datetime(s.time_created/1000, 'unixepoch', 'localtime') as created
FROM part p
JOIN session s ON s.id = p.session_id
WHERE json_extract(p.data, '$.type') = 'compaction'
GROUP BY p.session_id
ORDER BY compactions DESC
LIMIT 15;
```

### Todo completion rates per session
```sql
SELECT t.session_id, s.title,
       count(*) as total_todos,
       count(CASE WHEN t.status = 'completed' THEN 1 END) as completed,
       ROUND(100.0 * count(CASE WHEN t.status = 'completed' THEN 1 END) / count(*), 1) as completion_pct
FROM todo t
JOIN session s ON s.id = t.session_id
GROUP BY t.session_id
HAVING total_todos >= 3
ORDER BY completion_pct ASC
LIMIT 20;
```

### Sessions by OpenCode version
```sql
SELECT version, count(*) as sessions,
       datetime(min(time_created)/1000, 'unixepoch', 'localtime') as first_seen,
       datetime(max(time_created)/1000, 'unixepoch', 'localtime') as last_seen
FROM session
GROUP BY version
ORDER BY min(time_created) DESC;
```

## 6. Permissions

Permission events are recorded as `$.state.error` on tool parts with `status = 'error'`.
Four distinct signals: user rejection, rule-based denial, execution abort, question dismissal.

### Permission event summary (global counts)
```sql
SELECT
  count(CASE WHEN json_extract(data,'$.state.error') = 'Tool execution aborted' THEN 1 END) as aborted,
  count(CASE WHEN json_extract(data,'$.state.error') LIKE 'Error: The user rejected permission%' THEN 1 END) as user_rejected,
  count(CASE WHEN json_extract(data,'$.state.error') LIKE 'Error: The user has specified a rule which prevents%' THEN 1 END) as rule_blocked,
  count(CASE WHEN json_extract(data,'$.state.error') = 'Error: The user dismissed this question' THEN 1 END) as question_dismissed
FROM part
WHERE json_extract(data,'$.type') = 'tool'
  AND json_extract(data,'$.state.status') = 'error';
```

### Which tools get rejected most by users
```sql
SELECT json_extract(data,'$.tool') as tool, count(*) as rejections
FROM part
WHERE json_extract(data,'$.type') = 'tool'
  AND json_extract(data,'$.state.error') LIKE 'Error: The user rejected permission%'
GROUP BY tool
ORDER BY rejections DESC;
```

### Which tools get blocked by permission rules
```sql
SELECT json_extract(data,'$.tool') as tool, count(*) as blocked
FROM part
WHERE json_extract(data,'$.type') = 'tool'
  AND json_extract(data,'$.state.error') LIKE 'Error: The user has specified a rule which prevents%'
GROUP BY tool
ORDER BY blocked DESC;
```

### Sessions with most permission issues
```sql
SELECT p.session_id, s.title,
       datetime(s.time_created/1000, 'unixepoch', 'localtime') as created,
       count(CASE WHEN json_extract(p.data,'$.state.error') LIKE '%rejected permission%' THEN 1 END) as rejected,
       count(CASE WHEN json_extract(p.data,'$.state.error') LIKE '%rule which prevents%' THEN 1 END) as rule_blocked,
       count(CASE WHEN json_extract(p.data,'$.state.error') = 'Tool execution aborted' THEN 1 END) as aborted
FROM part p
JOIN session s ON s.id = p.session_id
WHERE json_extract(p.data,'$.type') = 'tool'
  AND json_extract(p.data,'$.state.status') = 'error'
  AND (json_extract(p.data,'$.state.error') LIKE '%rejected permission%'
    OR json_extract(p.data,'$.state.error') LIKE '%rule which prevents%'
    OR json_extract(p.data,'$.state.error') = 'Tool execution aborted')
GROUP BY p.session_id
ORDER BY (rejected + rule_blocked + aborted) DESC
LIMIT 15;
```

### What was the agent trying to do when blocked (inspect inputs)
```sql
SELECT json_extract(data,'$.tool') as tool,
       substr(json_extract(data,'$.state.input'), 1, 300) as attempted_input,
       datetime(time_created/1000, 'unixepoch', 'localtime') as when_blocked
FROM part
WHERE session_id = '<SESSION_ID>'
  AND json_extract(data,'$.type') = 'tool'
  AND (json_extract(data,'$.state.error') LIKE '%rejected permission%'
    OR json_extract(data,'$.state.error') LIKE '%rule which prevents%')
ORDER BY time_created;
```
