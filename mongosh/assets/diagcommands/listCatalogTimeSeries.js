try {
  const adminDB = db.getSiblingDB('admin');
  printjson(adminDB.aggregate([
    { $listCatalog: {} },
    { $match: { name: { $regex: '^system\\.buckets\\.' } } }
  ]).toArray());
} catch (e) {
  print("ERROR: " + e);
}
