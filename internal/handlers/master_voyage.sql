WITH SpecificVoyageService AS (
    SELECT
        VV.CARRIER_VOYAGE_KEY AS CURRENT_VOYAGE_KEY,
        VV.CARRIER_SERVICE_CODE AS SPECIFIC_SERVICE_CODE,
        VV.END_TIME AS MAIN_END_TIME
    FROM VESSEL_VOYAGE VV
             JOIN M_SEAFREIGHT_CARRIER SC ON VV.CARRIER_ID = SC.ID
             JOIN M_VESSEL V ON VV.VESSEL_ID = V.ID
    WHERE
      -- If voyage is provided, use it; otherwise, use startDate
        (VV.CARRIER_VOYAGE_KEY = :voyage OR :voyage IS NULL)
      AND (
        VV.START_TIME BETWEEN
            TO_DATE(:startDate, 'YYYY-MM-DD') - 30
            AND TO_DATE(:startDate,  'YYYY-MM-DD') + 30
            OR :startDate IS NULL
        )
      AND VV.IS_ACTIVE = 1
      AND V.IS_ACTIVE = 1
      AND SUBSTR(VV.DATA_SOURCE, 1, 3) NOT IN ('P44','OCE') --We only need direct carrier vessel voyage
      AND SC.CODE = :scac
      AND V.LLOYDS_CODE = :imo
        FETCH FIRST 1 ROWS ONLY
),
     NextVoyage AS (
         SELECT VV.CARRIER_VOYAGE_KEY AS FIRST_SUB_VOYAGE_KEY
         FROM VESSEL_VOYAGE VV
                  JOIN M_SEAFREIGHT_CARRIER SC ON VV.CARRIER_ID = SC.ID
                  JOIN M_VESSEL V ON VV.VESSEL_ID = V.ID
         WHERE SC.CODE = :scac
           AND V.LLOYDS_CODE = :imo
           AND VV.CARRIER_SERVICE_CODE = (SELECT SPECIFIC_SERVICE_CODE FROM SpecificVoyageService)
           AND VV.END_TIME > (SELECT MAIN_END_TIME FROM SpecificVoyageService)
           AND VV.CARRIER_VOYAGE_KEY != (SELECT CURRENT_VOYAGE_KEY FROM SpecificVoyageService)
    AND VV.IS_ACTIVE = 1
    AND V.IS_ACTIVE = 1
    AND SUBSTR(VV.DATA_SOURCE, 1, 3) NOT IN ('P44','OCE') --We only need direct carrier vessel voyage
ORDER BY VV.END_TIME ASC, VV.CARRIER_VOYAGE_KEY ASC
    FETCH FIRST 1 ROWS ONLY
    ),
    VesselVoyage AS (
        SELECT
            VV.DATA_SOURCE AS DATA_SOURCE,
            SC.CODE AS scac,
            VV.PROVIDER_VOYAGE_ID AS PROVIDER_VOYAGE_ID,
            VN.NAME AS VESSEL_NAME,
            V.LLOYDS_CODE AS VESSEL_IMO,
            VV.CARRIER_VOYAGE_KEY AS VOYAGE_NUM,
            VV.VOYAGE_DIRECTION AS VOYAGE_DIRECTION,
            VV.CARRIER_SERVICE_CODE AS SERVICE_CODE,
            GA.CODE AS PORT_CODE,
            GA.UN_INTERNATIONAL_NAME AS PORT_NAME,
            VVPT.EVENT AS PORT_EVENT,
            VVPT.TIME AS EVENT_TIME,
            ROW_NUMBER() OVER (PARTITION BY VV.CARRIER_VOYAGE_KEY, VVPT.EVENT, GA.CODE
            ORDER BY VV.PROVIDER_VOYAGE_ID, VV.UPDATE_TIME DESC) AS rnk
        FROM VESSEL_VOYAGE VV
            JOIN VESSEL_VOYAGE_PORT_STOP VVPT ON VV.ID = VVPT.VOYAGE_ID
            JOIN M_SEAFREIGHT_CARRIER SC ON VV.CARRIER_ID = SC.ID
            JOIN M_GEOGRAPHICAL_AREA GA ON VVPT.PORT_ID = GA.ID
            JOIN M_VESSEL V ON VV.VESSEL_ID = V.ID
            JOIN M_VESSEL_NAME VN ON V.ID = VN.VESSEL_ID
        WHERE VV.CARRIER_VOYAGE_KEY IN (
            (SELECT CURRENT_VOYAGE_KEY FROM SpecificVoyageService),
            (SELECT FIRST_SUB_VOYAGE_KEY FROM NextVoyage)
            )
          AND VV.IS_ACTIVE = 1
          AND V.IS_ACTIVE = 1
          AND VN.IS_ACTIVE = 1
          AND SUBSTR(VV.DATA_SOURCE, 1, 3) NOT IN ('P44','OCE') --We only need direct carrier vessel voyage
          AND SC.CODE = :scac
          AND V.LLOYDS_CODE = :imo
            )
        SELECT * FROM VesselVoyage
        WHERE rnk = 1 OR (PORT_EVENT = 'PAS' AND rnk < 3) -- remove duplicates by using rnk if any