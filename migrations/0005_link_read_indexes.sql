CREATE INDEX links_task_source_lookup_idx
ON links(source_task_id, target_kind, link_type, created_at DESC, id DESC)
WHERE target_kind = 'task';

CREATE INDEX links_task_target_lookup_idx
ON links(target_task_id, target_kind, link_type, created_at DESC, id DESC)
WHERE target_kind = 'task';

CREATE INDEX links_external_source_lookup_idx
ON links(source_task_id, target_kind, link_type, created_at DESC, id DESC)
WHERE target_kind = 'external';

CREATE INDEX links_external_global_lookup_idx
ON links(target_kind, created_at DESC, id DESC)
WHERE target_kind = 'external';
