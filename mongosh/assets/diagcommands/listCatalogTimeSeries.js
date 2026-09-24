try {
  const adminDB = db.getSiblingDB('admin');
  const buckets = adminDB.aggregate([
    { $listCatalog: {} },
    { $match: { name: { $regex: '^system\\.buckets\\.' } } }
  ]).toArray();
  printjson({ buckets: buckets });
} catch (e) {
  print("ERROR: " + e);
}
