// Adapted from mongodb/support-tools migration/toolbox/idChecker.
// Type counts use one $group per collection instead of one query per BSON type.
// Natural-order sampling (up to 1000 docs) still runs only for non-ObjectId _id types.
try {
  const maxCollections = (typeof _maxCollections !== "undefined") ? _maxCollections : 2500;
  const sampleLimit = 1000;
  const sampleShow = 8;
  const size30GB = 30 * 1024 * 1024 * 1024;
  const skipDb = { admin: 1, local: 1, config: 1 };
  const bsonBinarySubtypeUUID = 4;

  // Aggregation $type alias -> names used by the support-tools idChecker output.
  const typeAlias = {
    double: "Double",
    string: "String",
    object: "Object",
    array: "Array",
    binData: "Binary",
    undefined: "Undefined",
    objectId: "ObjectId",
    bool: "Boolean",
    date: "Date",
    null: "Null",
    regex: "Regex",
    dbPointer: "DBPointer",
    javascript: "JavaScript",
    symbol: "Symbol",
    javascriptWithScope: "JavaScriptWithScope",
    int: "Int32",
    timestamp: "Timestamp",
    long: "Int64",
    decimal: "Decimal128",
    minKey: "MinKey",
    maxKey: "MaxKey"
  };

  function isSequential(arr) {
    if (!Array.isArray(arr) || arr.length < 2) {
      return false;
    }
    for (let i = 1; i < arr.length; i++) {
      if (arr[i] <= arr[i - 1]) {
        return false;
      }
    }
    return true;
  }

  function isSequentialString(arr) {
    if (!Array.isArray(arr) || arr.length < 2) {
      return false;
    }
    const nums = arr.map(function (s) { return Number(s); });
    if (nums.some(isNaN)) {
      return false;
    }
    for (let i = 1; i < nums.length; i++) {
      if (nums[i] <= nums[i - 1]) {
        return false;
      }
    }
    return true;
  }

  function detectStringPattern(arr) {
    if (!Array.isArray(arr) || arr.length === 0) {
      return "unknown";
    }
    const sample = arr.slice(0, Math.min(10, arr.length));
    let uuidCount = 0;
    let numericCount = 0;
    sample.forEach(function (str) {
      if (typeof str !== "string") {
        return;
      }
      if (str.match(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i) ||
          str.match(/^[0-9a-f]{32}$/i)) {
        uuidCount++;
      } else if (str.match(/^\d+$/)) {
        numericCount++;
      }
    });
    if (uuidCount >= sample.length * 0.8) {
      return "UUID";
    }
    if (numericCount >= sample.length * 0.8) {
      return "Numeric";
    }
    return "Other";
  }

  function isSequentialDate(arr) {
    if (!Array.isArray(arr) || arr.length < 2) {
      return false;
    }
    const times = arr.map(function (d) { return d.getTime(); });
    for (let i = 1; i < times.length; i++) {
      if (times[i] <= times[i - 1]) {
        return false;
      }
    }
    return true;
  }

  function binaryToHex(bin) {
    if (bin && typeof bin.hex === "function") {
      return bin.hex();
    }
    if (bin && bin.buffer && typeof Buffer !== "undefined") {
      return Buffer.from(bin.buffer).toString("hex");
    }
    return String(bin);
  }

  function isBinaryUUID(ids) {
    if (!Array.isArray(ids) || ids.length === 0) {
      return false;
    }
    let uuidCount = 0;
    ids.forEach(function (id) {
      if (id && (id.subtype === bsonBinarySubtypeUUID || id.sub_type === bsonBinarySubtypeUUID)) {
        uuidCount++;
      }
    });
    return uuidCount >= ids.length * 0.8;
  }

  let mongoVersion;
  try {
    mongoVersion = db.serverBuildInfo().version;
  } catch (e) {
    mongoVersion = "" + e;
  }

  const listed = db.adminCommand({ listDatabases: 1, nameOnly: true });
  if (!listed || listed.ok !== 1) {
    throw (listed && (listed.errmsg || listed.codeName)) || "listDatabases failed";
  }
  const names = (listed.databases || []).map(function (d) { return d.name; });
  const collections = [];
  const errors = [];
  let collectionsExamined = 0;
  let truncated = false;

  for (let i = 0; i < names.length && !truncated; i++) {
    const dbName = names[i];
    if (skipDb[dbName]) {
      continue;
    }
    const sdb = db.getSiblingDB(dbName);
    let collInfos;
    try {
      collInfos = sdb.getCollectionInfos();
    } catch (e) {
      errors.push({ db: dbName, error: "" + e });
      continue;
    }
    for (let j = 0; j < collInfos.length; j++) {
      const info = collInfos[j];
      const collName = info.name;
      if (!collName || collName.indexOf("system.") === 0) {
        continue;
      }
      if (info.type && info.type !== "collection" && info.type !== "timeseries") {
        continue;
      }
      if (collectionsExamined >= maxCollections) {
        truncated = true;
        break;
      }
      collectionsExamined++;

      const namespace = dbName + "." + collName;
      try {
        const coll = sdb.getCollection(collName);
        let sizeBytes = 0;
        try {
          const collStats = coll.stats();
          if (collStats && typeof collStats.size === "number") {
            sizeBytes = collStats.size;
          }
        } catch (e) {
          sizeBytes = 0;
        }

        const grouped = coll.aggregate([
          { $group: { _id: { $type: "$_id" }, count: { $sum: 1 } } }
        ]).toArray();

        const idTypes = {};
        grouped.forEach(function (row) {
          const alias = row._id;
          const count = row.count || 0;
          if (!alias || count <= 0) {
            return;
          }
          const typeName = typeAlias[alias] || alias;
          idTypes[typeName] = {
            alias: alias,
            count: count,
            is_sequential: null,
            pattern: null,
            sample_ids: null
          };
        });

        const typeNames = Object.keys(idTypes);
        const hasOnlyObjectId = typeNames.length > 0 && typeNames.every(function (k) {
          return k === "ObjectId";
        });
        if (hasOnlyObjectId || typeNames.length === 0) {
          continue;
        }

        typeNames.forEach(function (typeName) {
          if (typeName === "ObjectId") {
            return;
          }
          const infoType = idTypes[typeName];
          const cursor = coll.find({ _id: { $type: infoType.alias } }, { _id: 1 }).sort({ $natural: 1 }).limit(sampleLimit);
          const ids = [];
          cursor.forEach(function (doc) { ids.push(doc._id); });

          if (typeName === "Int32" || typeName === "Double") {
            infoType.is_sequential = isSequential(ids);
            if (!infoType.is_sequential) {
              infoType.sample_ids = ids.slice(0, sampleShow);
            }
          } else if (typeName === "Int64" || typeName === "Decimal128" || typeName === "Timestamp") {
            infoType.is_sequential = "N/A";
            infoType.sample_ids = ids.slice(0, sampleShow);
          } else if (typeName === "String") {
            infoType.is_sequential = isSequentialString(ids);
            infoType.pattern = detectStringPattern(ids);
            if (!infoType.is_sequential) {
              infoType.sample_ids = ids.slice(0, sampleShow);
            }
          } else if (typeName === "Date") {
            infoType.is_sequential = isSequentialDate(ids);
            if (!infoType.is_sequential) {
              infoType.sample_ids = ids.slice(0, sampleShow);
            }
          } else if (typeName === "Binary") {
            if (isBinaryUUID(ids)) {
              infoType.is_sequential = false;
              infoType.pattern = "UUID";
              infoType.sample_ids = ids.slice(0, sampleShow).map(binaryToHex);
            } else {
              infoType.is_sequential = "N/A";
            }
          } else {
            infoType.is_sequential = "N/A";
          }
        });

        Object.keys(idTypes).forEach(function (k) {
          if (k === "ObjectId") {
            delete idTypes[k];
            return;
          }
          delete idTypes[k].alias;
          if (k !== "String" && k !== "Binary") {
            delete idTypes[k].pattern;
          }
          if (idTypes[k].is_sequential === true) {
            delete idTypes[k].sample_ids;
          }
          if (idTypes[k].sample_ids === null) {
            delete idTypes[k].sample_ids;
          }
        });

        if (Object.keys(idTypes).length === 0) {
          continue;
        }
        const hasNonSequential = Object.keys(idTypes).some(function (k) {
          return idTypes[k].is_sequential === false;
        });
        collections.push({
          namespace: namespace,
          database: dbName,
          collection: collName,
          collection_size_bytes: sizeBytes,
          id_types: idTypes,
          copyInNaturalOrder_recommended: hasNonSequential && sizeBytes >= size30GB
        });
      } catch (e) {
        errors.push({ db: dbName, collection: collName, error: "" + e });
      }
    }
  }

  const byDb = {};
  const slowMigration = [];
  const efficientMigration = [];
  collections.forEach(function (res) {
    Object.keys(res.id_types).forEach(function (typeName) {
      const typeInfo = res.id_types[typeName];
      let displayName = typeName;
      if ((typeName === "String" || typeName === "Binary") && typeInfo.pattern) {
        displayName = typeName + ":" + typeInfo.pattern;
      }
      if (typeInfo.is_sequential === false) {
        slowMigration.push(res.namespace + " (" + displayName + ")");
        if (!byDb[res.database]) {
          byDb[res.database] = [];
        }
        if (byDb[res.database].indexOf(res.collection) === -1) {
          byDb[res.database].push(res.collection);
        }
      } else if (typeInfo.is_sequential === true) {
        efficientMigration.push(res.namespace + " (" + displayName + ")");
      }
    });
  });
  const copyInNaturalOrder = Object.keys(byDb).map(function (dbName) {
    return { database: dbName, collections: byDb[dbName] };
  });

  printjson({
    source: "mongodb/support-tools migration/toolbox/idChecker",
    mongoVersion: mongoVersion,
    collectionsExamined: collectionsExamined,
    maxCollections: maxCollections,
    truncated: truncated,
    allObjectId: collections.length === 0 && errors.length === 0 && !truncated,
    collections: collections,
    copyInNaturalOrder: copyInNaturalOrder,
    slowMigration: slowMigration,
    efficientMigration: efficientMigration,
    errors: errors
  });
  if (errors.length) {
    print("ERROR: " + errors.length + " _id type check(s) failed");
  }
} catch (e) {
  print("ERROR: " + e);
}
