WITH CapStartTimeFromFirstVV AS ( -- Each CTE operation depenedent on one another. get first vv -> get next vv based on first vv -> combine first + next vv
    SELECT /*+ PARALLEL(VV, 8) PARALLEL(SC, 8) PARALLEL(V, 8) PARALLEL(VVPT, 8) */
        VV.ID AS VV_ID
    FROM VESSEL_VOYAGE VV
             JOIN M_SEAFREIGHT_CARRIER SC ON VV.CARRIER_ID = SC.ID
             JOIN M_VESSEL V ON VV.VESSEL_ID = V.ID
             JOIN VESSEL_VOYAGE_PORT_STOP VVPT ON VV.ID = VVPT.VOYAGE_ID
    WHERE (:voyage IS NULL OR VV.CARRIER_VOYAGE_KEY = :voyage)
      AND VV.IS_ACTIVE = 1
      AND V.IS_ACTIVE = 1
      AND SUBSTR(VV.DATA_SOURCE, 1, 3) NOT IN ('P44', 'OCE') -- Exclude non-direct carrier vessel voyages
      AND SC.CODE = :scac
      AND V.LLOYDS_CODE = :imo
      AND (:startDate IS NULL OR VVPT.TIME BETWEEN TO_DATE(:startDate, 'YYYY-MM-DD') - :dateRange
      AND TO_DATE(:startDate, 'YYYY-MM-DD') + :dateRange)
    ORDER BY VV.UPDATE_TIME DESC, VV.START_TIME ASC
        FETCH FIRST 1 ROWS ONLY
),
     FirstVoyage AS (
         SELECT /*+ PARALLEL(VV, 8) */
             VV.CARRIER_SERVICE_CODE AS MAIN_SERVICE_CODE,
             VV.END_TIME AS MAIN_END_TIME,
             CEIL((CAST(VV.END_TIME AS DATE) - CAST(VV.START_TIME AS DATE))) AS MAIN_TT
         FROM VESSEL_VOYAGE VV
         WHERE VV.ID = (SELECT VV_ID FROM CapStartTimeFromFirstVV)
     ),
     NextVoyage AS (
         SELECT /*+ PARALLEL(VV, 8) PARALLEL(SC, 8) PARALLEL(V, 8) */
             VV.ID AS VV_SUB_ID,
             VV.CARRIER_VOYAGE_KEY AS FIRST_SUB_VOYAGE_KEY
         FROM VESSEL_VOYAGE VV
                  JOIN M_SEAFREIGHT_CARRIER SC ON VV.CARRIER_ID = SC.ID
                  JOIN M_VESSEL V ON VV.VESSEL_ID = V.ID
                  CROSS JOIN FirstVoyage FV -- Use CROSS JOIN to avoid repeated subqueries
         WHERE SC.CODE = :scac
           AND V.LLOYDS_CODE = :imo
           AND VV.CARRIER_SERVICE_CODE = FV.MAIN_SERVICE_CODE
           AND VV.START_TIME BETWEEN CAST(FV.MAIN_END_TIME AS DATE) - FV.MAIN_TT AND CAST(FV.MAIN_END_TIME AS DATE) + FV.MAIN_TT
           AND VV.END_TIME > FV.MAIN_END_TIME
           AND VV.ID != (SELECT VV_ID FROM CapStartTimeFromFirstVV)
    AND VV.IS_ACTIVE = 1
    AND V.IS_ACTIVE = 1
    AND SUBSTR(VV.DATA_SOURCE, 1, 3) NOT IN ('P44', 'OCE') -- Exclude non-direct carrier vessel voyages
ORDER BY VV.UPDATE_TIME DESC, VV.END_TIME ASC, VV.CARRIER_VOYAGE_KEY ASC
    FETCH FIRST 1 ROWS ONLY
    ),
    CombinedVesselVoyage AS (
        SELECT /*+ PARALLEL(VV, 8) PARALLEL(VVPT, 8) PARALLEL(SC, 8) PARALLEL(GA, 8) PARALLEL(V, 8) PARALLEL(VN, 8) */
            VV.DATA_SOURCE,
            SC.CODE AS scac,
            VV.PROVIDER_VOYAGE_ID,
            VN.NAME AS VESSEL_NAME,
            V.LLOYDS_CODE AS VESSEL_IMO,
            VV.CARRIER_VOYAGE_KEY AS VOYAGE_NUM,
            VV.VOYAGE_DIRECTION,
            VV.CARRIER_SERVICE_CODE AS SERVICE_CODE,
            GA.CODE AS PORT_CODE,
            GA.UN_INTERNATIONAL_NAME AS PORT_NAME,
            VVPT.EVENT AS PORT_EVENT,
            VVPT.TIME AS EVENT_TIME
        FROM VESSEL_VOYAGE VV
            JOIN VESSEL_VOYAGE_PORT_STOP VVPT ON VV.ID = VVPT.VOYAGE_ID
            JOIN M_SEAFREIGHT_CARRIER SC ON VV.CARRIER_ID = SC.ID
            JOIN M_GEOGRAPHICAL_AREA GA ON VVPT.PORT_ID = GA.ID
            JOIN M_VESSEL V ON VV.VESSEL_ID = V.ID
            JOIN M_VESSEL_NAME VN ON V.ID = VN.VESSEL_ID
        WHERE VV.ID IN (
            (SELECT VV_ID FROM CapStartTimeFromFirstVV),
            (SELECT VV_SUB_ID FROM NextVoyage)
            )
            )
        SELECT *
        FROM CombinedVesselVoyage