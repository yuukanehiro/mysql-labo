#!/bin/bash
for i in {1..30}; do
  echo "------ $(date) ------" >> locks.txt
  mysql -uroot -proot -h127.0.0.1 -e \
    "SELECT THREAD_ID, INDEX_NAME, OBJECT_NAME, LOCK_TYPE, LOCK_MODE, LOCK_STATUS, LOCK_DATA FROM performance_schema.data_locks" \
    >> locks.txt
  sleep 0.5
done