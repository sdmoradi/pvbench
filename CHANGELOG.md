# Changelog

## 1.0.0 (2026-11-03)


### Features

* **benchmark:** add benchmark job runner and cleanup ([4d98f90](https://github.com/sdmoradi/pvbench/commit/4d98f90622c2eaf293ddda0619c366b99a05ff50))
* **benchmark:** add storage class validation ([11ecd69](https://github.com/sdmoradi/pvbench/commit/11ecd69e905c8c2e6137de8598a259c7ff5d3304))
* **benchmark:** create fio benchmark jobs ([59815c6](https://github.com/sdmoradi/pvbench/commit/59815c656172a49f4d0901f31f98250b2bea64cf))
* **benchmark:** create temporary benchmark PVCs ([23d7cd7](https://github.com/sdmoradi/pvbench/commit/23d7cd7a372d3f18819c3696479e45ac56625c24))
* **benchmark:** parse and render FIO summary ([79f7d96](https://github.com/sdmoradi/pvbench/commit/79f7d96e14580b62616e7da3b5cace09c691adb7))
* **benchmark:** render summary and report failed workloads ([d4504a6](https://github.com/sdmoradi/pvbench/commit/d4504a6329c46ecc693dd6c5b829838b816a3dbc))
* **cli:** register benchmark command ([107ec1e](https://github.com/sdmoradi/pvbench/commit/107ec1ee9e19e24bd96c220c2417e4e4e2a7413f))
* **inspect:** add Kubernetes PVC inspection ([8bff3ee](https://github.com/sdmoradi/pvbench/commit/8bff3ee36c6093c7d23a9383554c0438e2d1cb47))


### Bug Fixes

* **benchmark:** correct FIO JSON struct tags ([eb2abca](https://github.com/sdmoradi/pvbench/commit/eb2abca24f67bc3b771703108852872e5d935f7f))
* **benchmark:** find job pods using compatible labels ([0a4482a](https://github.com/sdmoradi/pvbench/commit/0a4482ae89c4ae3fafb0b3bae3ac17a5300f5c76))
* **benchmark:** parse job resource quantities ([20b58ac](https://github.com/sdmoradi/pvbench/commit/20b58ac6c8cba8cbb5392095c2374820b06e111e))
* **benchmark:** show failed rows when logs are unavailable ([3884d0a](https://github.com/sdmoradi/pvbench/commit/3884d0aa3dd210a7353fb90e8781dbeca39cdb70))
* **ci:** use manifest-based release initialization ([883eeee](https://github.com/sdmoradi/pvbench/commit/883eeee5eb4aa617a8cb427b91991713743b22dc))
