try {
  const skipDb = { admin: 1, local: 1, config: 1 };
  let mongoVersion;
  try {
    mongoVersion = db.serverBuildInfo().version;
  } catch (e) {
    mongoVersion = '' + e;
  }
  let fcv;
  try {
    fcv = db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 }).featureCompatibilityVersion;
  } catch (e) {
    fcv = { error: '' + e };
  }
  const listed = db.adminCommand({ listDatabases: 1, nameOnly: true });
  if (!listed || listed.ok !== 1) {
    throw (listed && (listed.errmsg || listed.codeName)) || "listDatabases failed";
  }
  const names = (listed.databases || []).map(function (d) { return d.name; });
  const indexes = [];
  const errors = [];
  names.forEach(function (dbName) {
    if (skipDb[dbName]) {
      return;
    }
    const sdb = db.getSiblingDB(dbName);
    let collInfos;
    try {
      collInfos = sdb.getCollectionInfos();
    } catch (e) {
      errors.push({ db: dbName, error: '' + e });
      return;
    }
    collInfos.forEach(function (info) {
      const collName = info.name;
      if (!collName || collName.indexOf('system.') === 0) {
        return;
      }
      if (info.type && info.type !== 'collection' && info.type !== 'timeseries') {
        return;
      }
      try {
        const idxList = sdb.getCollection(collName).getIndexes();
        const unique = [];
        idxList.forEach(function (idx) {
          if (idx.unique && idx.name !== '_id_') {
            unique.push(idx);
          }
        });
        if (!unique.length) {
          return;
        }
        const stats = sdb.getCollection(collName).aggregate([
          { $collStats: { storageStats: {} } },
          { $project: { 'storageStats.indexDetails': 1 } }
        ]).toArray();
        const details = (stats[0] && stats[0].storageStats && stats[0].storageStats.indexDetails) || {};
        unique.forEach(function (idx) {
          const meta = details[idx.name] && details[idx.name].metadata;
          const formatVersion = meta ? meta.formatVersion : undefined;
          const newFormat = formatVersion === 13 || formatVersion === 14;
          indexes.push({
            db: dbName,
            collection: collName,
            index: idx.name,
            key: idx.key,
            formatVersion: formatVersion,
            newFormat: newFormat
          });
        });
      } catch (e) {
        errors.push({ db: dbName, collection: collName, error: '' + e });
      }
    });
  });
  const oldFormat = indexes.filter(function (i) { return !i.newFormat; });
  printjson({
    mongoVersion: mongoVersion,
    featureCompatibilityVersion: fcv,
    uniqueIndexes: indexes,
    oldFormat: oldFormat,
    allNewFormat: oldFormat.length === 0,
    errors: errors
  });
  if (errors.length) {
    print("ERROR: " + errors.length + " unique-index check(s) failed");
  }
} catch (e) {
  print("ERROR: " + e);
}
