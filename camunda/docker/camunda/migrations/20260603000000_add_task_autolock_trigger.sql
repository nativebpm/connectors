-- Create a trigger function to automatically lock external tasks for the CDC worker
CREATE OR REPLACE FUNCTION public.lock_camunda_external_task()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.worker_id_ IS NULL AND NEW.topic_name_ IN ('creditScoreChecker', 'decider', 'loanGranter', 'requestRejecter') THEN
    NEW.worker_id_ := 'loan-worker-cdc';
    NEW.lock_exp_time_ := NOW() + INTERVAL '5 minutes';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Bind the trigger function BEFORE INSERT OR UPDATE on act_ru_ext_task
CREATE OR REPLACE TRIGGER lock_camunda_external_task_trigger
BEFORE INSERT OR UPDATE ON public.act_ru_ext_task
FOR EACH ROW
EXECUTE FUNCTION public.lock_camunda_external_task();
