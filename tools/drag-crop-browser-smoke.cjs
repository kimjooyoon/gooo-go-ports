// Run both real compiled consumers in one JS host: the corner mode must not
// replace the legacy bridge or normalize original inputs in JavaScript.
const assert = require('node:assert/strict');
const { runSmoke } = require('./crop-browser-smoke.cjs');

async function runBoth(directory) {
  const ordered = await runSmoke(directory);
  const corners = await runSmoke(directory, 'drag-crop');
  const bounds = ['0', '0', '640', '480'];
  const selections = [
    ['100', '100', '200', '200'], ['200', '100', '100', '200'],
    ['100', '200', '200', '100'], ['200', '200', '100', '100']
  ];
  const empty = { min_x: '0', min_y: '0', max_x: '0', max_y: '0', has_pixels: false };
  const area = { min_x: '100', min_y: '100', max_x: '200', max_y: '200', has_pixels: true };
  const compare = (inputs, cornerOutput, orderedOutput) => {
    for (const [fn, output] of [[globalThis.goooDragCrop, cornerOutput], [globalThis.goooCrop, orderedOutput]]) {
      assert.deepEqual(JSON.parse(fn(...inputs)), { ok: true, inputs, output });
    }
  };
  selections.forEach((selection, i) => compare([...bounds, ...selection], area, i === 0 ? area : empty));
  compare(['640', '0', '0', '480', ...selections[3]], empty, empty);
  compare(['-9223372036854775808', '-9223372036854775808', '9223372036854775807', '9223372036854775807',
    '9223372036854775807', '-9223372036854775807', '9223372036854775806', '-9223372036854775808'],
    { min_x: '9223372036854775806', min_y: '-9223372036854775808', max_x: '9223372036854775807', max_y: '-9223372036854775807', has_pixels: true }, empty);
  compare(['9007199254740992', '0', '9007199254740994', '10', '9007199254740995', '8', '9007199254740993', '2'],
    { min_x: '9007199254740993', min_y: '2', max_x: '9007199254740994', max_y: '8', has_pixels: true }, empty);
  return { schema: 'gooo/dual-crop-browser-smoke/v1', status: 'PASS', ordered, corners,
    paired_cases: 7, paired_calls: 14,
    scope: 'Both actual compiled consumers coexist; original strings echoed and manual boundary outputs compared. No browser UI/layout or pixel processing claim.' };
}
module.exports = { runBoth };
if (require.main === module) {
  runBoth(process.argv[2] || 'out/browser/site').then(result => {
    console.log(JSON.stringify(result)); process.exit(0);
  }).catch(error => { console.error(error); process.exit(1); });
}
