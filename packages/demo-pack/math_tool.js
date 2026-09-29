/**
 * @name add_numbers
 * @description Adds two numbers together
 * @mcp true
 * @param {number} a - First number
 * @param {number} b - Second number
 */
function main(params) {
    const a = (params && params.a) || 0;
    const b = (params && params.b) || 0;
    return { result: a + b };
}
