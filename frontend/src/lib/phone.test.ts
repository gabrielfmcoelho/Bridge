// Run: node --test src/lib/phone.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { formatPhone, phoneProblem, whatsappNumber } from "./phone.ts";

test("formats mobile, landline and country code", () => {
  assert.equal(formatPhone("86999998888"), "(86) 9 9999-8888");
  assert.equal(formatPhone("8632211234"), "(86) 3221-1234");
  assert.equal(formatPhone("5586999998888"), "+55 (86) 9 9999-8888");
  assert.equal(formatPhone("(86) 9 9999-8888"), "(86) 9 9999-8888");
});

test("formats partial input as it's typed", () => {
  assert.equal(formatPhone("8"), "(8");
  assert.equal(formatPhone("869"), "(86) 9");
  assert.equal(formatPhone("869999"), "(86) 9999");
});

test("problems and WhatsApp number", () => {
  assert.equal(phoneProblem(""), "");
  assert.equal(phoneProblem("86999"), "contact.phoneTooShort");
  assert.equal(phoneProblem("86999998888"), "");
  assert.equal(whatsappNumber("86999998888"), "5586999998888");
  assert.equal(whatsappNumber("5586999998888"), "5586999998888");
});
