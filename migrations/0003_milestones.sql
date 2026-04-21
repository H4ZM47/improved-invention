CREATE TABLE milestones (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  handle TEXT NOT NULL UNIQUE CHECK(handle GLOB 'MILE-[0-9]*'),
  name TEXT NOT NULL CHECK(length(trim(name)) > 0),
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'backlog' CHECK(status IN ('backlog', 'active', 'paused', 'blocked', 'completed', 'cancelled')),
  due_at TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  closed_at TEXT
);

ALTER TABLE tasks
ADD COLUMN milestone_id INTEGER REFERENCES milestones(id) ON DELETE RESTRICT;

CREATE INDEX tasks_milestone_lookup_idx
ON tasks(milestone_id);

CREATE TABLE events_new (
  id INTEGER PRIMARY KEY,
  uuid TEXT NOT NULL UNIQUE,
  entity_type TEXT NOT NULL CHECK(entity_type IN ('task', 'project', 'domain', 'milestone', 'actor', 'claim', 'view', 'system')),
  entity_uuid TEXT NOT NULL CHECK(length(trim(entity_uuid)) > 0),
  actor_id INTEGER REFERENCES actors(id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL CHECK(length(trim(event_type)) > 0),
  payload_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(payload_json)),
  occurred_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO events_new(id, uuid, entity_type, entity_uuid, actor_id, event_type, payload_json, occurred_at)
SELECT id, uuid, entity_type, entity_uuid, actor_id, event_type, payload_json, occurred_at
FROM events;

DROP TABLE events;

ALTER TABLE events_new RENAME TO events;

CREATE INDEX events_entity_lookup_idx
ON events(entity_type, entity_uuid, occurred_at);

CREATE INDEX events_actor_lookup_idx
ON events(actor_id, occurred_at);

CREATE TABLE handle_sequences_new (
  entity_type TEXT PRIMARY KEY CHECK(entity_type IN ('task', 'project', 'domain', 'milestone', 'actor')),
  next_value INTEGER NOT NULL CHECK(next_value > 0)
);

INSERT INTO handle_sequences_new(entity_type, next_value)
SELECT entity_type, next_value
FROM handle_sequences;

INSERT INTO handle_sequences_new(entity_type, next_value)
VALUES ('milestone', 1);

DROP TABLE handle_sequences;

ALTER TABLE handle_sequences_new RENAME TO handle_sequences;
