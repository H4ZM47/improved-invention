CREATE TABLE actors (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL UNIQUE CHECK(handle GLOB 'ACT-[0-9]*'),
  kind TEXT NOT NULL CHECK(kind IN ('human', 'agent')),
  provider TEXT,
  external_id TEXT NOT NULL CHECK(length(trim(external_id)) > 0),
  display_name TEXT NOT NULL DEFAULT '',
  first_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK(
    (kind = 'human' AND (provider IS NULL OR length(trim(provider)) = 0))
    OR
    (kind = 'agent' AND provider IS NOT NULL AND length(trim(provider)) > 0)
  )
);

CREATE UNIQUE INDEX actors_identity_lookup_idx
ON actors(kind, ifnull(provider, ''), external_id);

CREATE TABLE domains (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL UNIQUE CHECK(handle GLOB 'DOM-[0-9]*'),
  name TEXT NOT NULL CHECK(length(trim(name)) > 0),
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'backlog' CHECK(status IN ('backlog', 'active', 'paused', 'blocked', 'completed', 'cancelled')),
  default_assignee_actor_id INTEGER REFERENCES actors(id) ON DELETE RESTRICT,
  assignee_actor_id INTEGER REFERENCES actors(id) ON DELETE RESTRICT,
  due_at TEXT,
  tags TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags)),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  closed_at TEXT
);

CREATE TABLE projects (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL UNIQUE CHECK(handle GLOB 'PROJ-[0-9]*'),
  name TEXT NOT NULL CHECK(length(trim(name)) > 0),
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'backlog' CHECK(status IN ('backlog', 'active', 'paused', 'blocked', 'completed', 'cancelled')),
  domain_id INTEGER NOT NULL REFERENCES domains(id) ON DELETE RESTRICT,
  default_assignee_actor_id INTEGER REFERENCES actors(id) ON DELETE RESTRICT,
  assignee_actor_id INTEGER REFERENCES actors(id) ON DELETE RESTRICT,
  due_at TEXT,
  tags TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags)),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  closed_at TEXT
);

CREATE TABLE tasks (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL UNIQUE CHECK(handle GLOB 'TASK-[0-9]*'),
  title TEXT NOT NULL CHECK(length(trim(title)) > 0),
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'backlog' CHECK(status IN ('backlog', 'active', 'paused', 'blocked', 'completed', 'cancelled')),
  domain_id INTEGER REFERENCES domains(id) ON DELETE RESTRICT,
  project_id INTEGER REFERENCES projects(id) ON DELETE RESTRICT,
  assignee_actor_id INTEGER REFERENCES actors(id) ON DELETE RESTRICT,
  due_at TEXT,
  tags TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags)),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  closed_at TEXT
);

CREATE TABLE claims (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  actor_id INTEGER NOT NULL REFERENCES actors(id) ON DELETE RESTRICT,
  claimed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at TEXT NOT NULL,
  renewed_at TEXT,
  released_at TEXT,
  release_reason TEXT
);

CREATE UNIQUE INDEX claims_one_open_claim_per_task_idx
ON claims(task_id)
WHERE released_at IS NULL;

CREATE TABLE relationships (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  source_task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  target_task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  relationship_type TEXT NOT NULL CHECK(relationship_type IN ('parent_child', 'blocks', 'related_to', 'sibling_of', 'duplicate_of', 'supersedes')),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK(source_task_id <> target_task_id)
);

CREATE UNIQUE INDEX relationships_unique_edge_idx
ON relationships(source_task_id, target_task_id, relationship_type);

CREATE UNIQUE INDEX relationships_one_parent_per_child_idx
ON relationships(target_task_id)
WHERE relationship_type = 'parent_child';

CREATE TABLE external_links (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
  project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
  domain_id INTEGER REFERENCES domains(id) ON DELETE CASCADE,
  link_type TEXT NOT NULL CHECK(link_type IN ('file', 'url', 'repo', 'worktree', 'obsidian', 'other')),
  target TEXT NOT NULL CHECK(length(trim(target)) > 0),
  label TEXT NOT NULL DEFAULT '',
  metadata_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(metadata_json)),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK(
    (CASE WHEN task_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN project_id IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN domain_id IS NULL THEN 0 ELSE 1 END) = 1
  )
);

CREATE UNIQUE INDEX external_links_unique_target_idx
ON external_links(
  ifnull(task_id, 0),
  ifnull(project_id, 0),
  ifnull(domain_id, 0),
  link_type,
  target
);

CREATE TABLE events (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  entity_type TEXT NOT NULL CHECK(entity_type IN ('task', 'project', 'domain', 'actor', 'claim', 'view', 'system')),
  entity_uuid TEXT NOT NULL CHECK(length(trim(entity_uuid)) > 0),
  actor_id INTEGER REFERENCES actors(id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL CHECK(length(trim(event_type)) > 0),
  payload_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(payload_json)),
  occurred_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX events_entity_lookup_idx
ON events(entity_type, entity_uuid, occurred_at);

CREATE INDEX events_actor_lookup_idx
ON events(actor_id, occurred_at);

CREATE TABLE saved_views (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL UNIQUE CHECK(length(trim(name)) > 0),
  entity_type TEXT NOT NULL DEFAULT 'task' CHECK(entity_type IN ('task')),
  filters_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(filters_json)),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE handle_sequences (
  entity_type TEXT PRIMARY KEY CHECK(entity_type IN ('task', 'project', 'domain', 'actor')),
  next_value INTEGER NOT NULL CHECK(next_value > 0)
);

INSERT INTO handle_sequences(entity_type, next_value) VALUES
  ('task', 1),
  ('project', 1),
  ('domain', 1),
  ('actor', 1);
