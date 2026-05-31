-- atlas:txmode none

-- Create custom task queue table
CREATE TABLE IF NOT EXISTS custom_task_queue (
    id SERIAL PRIMARY KEY,
    task_type VARCHAR(255) NOT NULL,
    payload TEXT NOT NULL,
    workflow_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Alter table to set REPLICA IDENTITY FULL
ALTER TABLE custom_task_queue REPLICA IDENTITY FULL;

-- Create publication for Sequin
CREATE PUBLICATION sequin_pub FOR TABLE custom_task_queue;

-- Create replication slot if it doesn't exist
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_replication_slots WHERE slot_name = 'sequin_slot'
  ) THEN
    PERFORM pg_create_logical_replication_slot('sequin_slot', 'pgoutput');
  END IF;
END
$$;

