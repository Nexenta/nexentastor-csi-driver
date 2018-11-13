#!/usr/bin/env bash

#
# Run configured count of nginx containers.
# Usage:
#   deploy-stress-test.sh <apply|delete> <COUNTAINER_COUNT>
#

OPERATION=$1
COUNTAINER_COUNT=$2

USAGE="Usage:\ndeploy-stress-test.sh <apply|delete> <COUNTAINER_COUNT>";
FILE="./deploy-stress-test.yaml"

if [ -z "${OPERATION}" ] || [ -z "${COUNTAINER_COUNT}" ]; then
    echo -e "${USAGE}";
    exit 1;
fi;

COUNTER=0
while [ $COUNTER -lt $COUNTAINER_COUNT ]; do
    let COUNTER=COUNTER+1;
    sed -i -e "s/-auto.*$/-auto-${COUNTER}/g" "${FILE}";
    echo "${OPERATION}: ${COUNTER} of ${COUNTAINER_COUNT}...";
    if [ "${OPERATION}" = "apply" ]; then
        kubectl "${OPERATION}" -f "${FILE}";
        sleep 0.1;
    else
        kubectl "${OPERATION}" -f "${FILE}" &
        sleep 0.5;
    fi
done

sed -i -e "s/-auto.*$/-auto/g" "${FILE}";
