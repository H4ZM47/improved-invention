CREATE TABLE links (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  source_task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  link_type TEXT NOT NULL CHECK(link_type IN (
    'parent_child',
    'blocks',
    'related_to',
    'sibling_of',
    'duplicate_of',
    'supersedes',
    'file',
    'url',
    'repo',
    'worktree',
    'obsidian',
    'other'
  )),
  target_kind TEXT NOT NULL CHECK(target_kind IN ('task', 'external')),
  target_task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
  target_value TEXT,
  label TEXT NOT NULL DEFAULT '',
  metadata_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(metadata_json)),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK(
    (target_kind = 'task' AND target_task_id IS NOT NULL AND target_value IS NULL)
    OR
    (target_kind = 'external' AND target_task_id IS NULL AND target_value IS NOT NULL AND length(trim(target_value)) > 0)
  ),
  CHECK(source_task_id <> ifnull(target_task_id, -1))
);

CREATE UNIQUE INDEX links_unique_task_edge_idx
ON links(source_task_id, target_task_id, link_type)
WHERE target_kind = 'task';

CREATE UNIQUE INDEX links_one_parent_per_child_idx
ON links(target_task_id)
WHERE target_kind = 'task' AND link_type = 'parent_child';

CREATE UNIQUE INDEX links_unique_external_target_idx
ON links(source_task_id, link_type, target_value)
WHERE target_kind = 'external';

INSERT INTO links(uuid, source_task_id, link_type, target_kind, target_task_id, target_value, label, metadata_json, created_at)
SELECT
  uuid,
  source_task_id,
  relationship_type,
  'task',
  target_task_id,
  NULL,
  '',
  '{}',
  created_at
FROM relationships;

INSERT INTO links(uuid, source_task_id, link_type, target_kind, target_task_id, target_value, label, metadata_json, created_at)
SELECT
  uuid,
  task_id,
  link_type,
  'external',
  NULL,
  target,
  label,
  metadata_json,
  created_at
FROM external_links
WHERE task_id IS NOT NULL;

DROP TABLE relationships;
DROP TABLE external_links;
